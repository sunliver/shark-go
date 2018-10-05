package proxy

import (
	"io"
	"net"
	"time"

	uuid "github.com/satori/go.uuid"
	client "github.com/sunliver/shark-go/client/localclient"
	"github.com/sunliver/shark-go/protocol"

	log "github.com/sirupsen/logrus"
)

// conn status
const (
	constStatusHandShake = iota
	constStatusConnected
	constStatusClosed
)

// proxyClient handle connection from local
// send messages via localclient
type proxyClient struct {
	ID        uuid.UUID
	c         *client.Client
	conn      net.Conn
	proxy     proxy
	writechan chan *protocol.BlockData
	status    int
}

const (
	constReadTimeoutS = time.Second * 60
)

const (
	constProxyTypeHTTP = iota
	constProxyTypeHTTPS
	constProxyTypeSocks4
	constProxyTypeSocks5
)

type proxy interface {
	// HandShake returns proxy handshake msg
	handShake(conn net.Conn) (blockdata *protocol.BlockData, read []byte, err error)
	// HandShakeResp returns proxy handshake resp msg
	handShakeResp() []byte
	proxyType() int
}

type hostData struct {
	Address string `json:"Address"`
	Port    uint16 `json:"Port"`
}

// ServerProxy handle proxy connections
func ServerProxy(p string, conn net.Conn, c *client.Client) {
	// TODO socks support
	p = "http"
	pc := &proxyClient{
		ID:        protocol.NewGUID(),
		proxy:     &httpProxy{},
		c:         c,
		conn:      conn,
		writechan: make(chan *protocol.BlockData),
	}

	for pc.c.RegisterObserver(pc.ID, pc.onRead) != nil {
		pc.ID = protocol.NewGUID()
	}
	pc.start()
}

// start proxyClient start accept requests
func (pc *proxyClient) start() {
	blockdata, remain, err := pc.proxy.handShake(pc.conn)
	if err != nil && err != errHTTPDegrade {
		log.Errorf("[proxyClient] get proxy handshake msg failed, err: %v", err)
		pc.release()
		return
	}

	log.Infof("[proxyClient] send handshake msg, %v", string(blockdata.Data))

	blockdata.ID = pc.ID
	if _, err := pc.c.Write(blockdata); err != nil {
		log.Warnf("[proxyClient] send handshake msg failed, err: %v", err)
		pc.release()
		return
	}

	// wait connected resp
	data := <-pc.writechan

	if data.Type == protocol.ConstBlockTypeConnected {
		if pc.proxy.proxyType() == constProxyTypeHTTPS {
			if err := pc.writeToLocal(pc.proxy.handShakeResp()); err != nil {
				pc.release()
				return
			}
			log.Infof("[proxyClient] write handshake resp")
		}
	} else if data.Type == protocol.ConstBlockTypeConnectFailed {
		log.Infof("[proxyClient] recv connect failed, %s", data)
		pc.release()
		return
	} else {
		log.Errorf("[proxyClient] unknown data, %v:%v:%v:%v", blockdata.ID, blockdata.Type, blockdata.BlockNum, blockdata.Length)
		pc.release()
		return
	}

	// send http remain
	if _, err := pc.c.Write(&protocol.BlockData{
		ID:   pc.ID,
		Type: protocol.ConstBlockTypeData,
		Data: remain,
	}); err != nil {
		log.Errorf("[proxyClient] send http msg failed, err: %v", err)
		pc.release()
		return
	}

	go pc.beginRead()
	go pc.write()
}

func (pc *proxyClient) release() {
	if pc.status != constStatusClosed {
		pc.c.UnRegisterObserver(pc.ID)
		pc.conn.Close()
		pc.status = constStatusClosed
	}
	log.Infof("[proxyClient] %v released", pc.ID)
}

func (pc *proxyClient) beginRead() {
	log.Debugf("[proxyClient] %v begin read", pc.ID)
	defer pc.release()

	for {
		// if err := pc.conn.SetReadDeadline(time.Now().Add(constReadTimeoutS)); err != nil {
		// 	log.Warnf("[proxyClient] read timeout, err: %v", err)
		// 	break
		// }

		buf := make([]byte, 4096)
		n, err := io.ReadAtLeast(pc.conn, buf, 1)
		if err != nil {
			log.Infof("[proxyClient] read from local failed, err: %v", err)
			break
		}

		if _, err := pc.c.Write(&protocol.BlockData{
			ID:   pc.ID,
			Type: protocol.ConstBlockTypeData,
			Data: buf[:n],
		}); err != nil {
			log.Warnf("[proxyClient] write data to remote failed, err: %v", err)
			break
		}
	}
}

func (pc *proxyClient) onRead(data *protocol.BlockData, err error) {
	if err != nil {
		log.Infof("[proxyClient] client is closed, err: %v", err)
		pc.release()
		return
	}

	pc.writechan <- data
}

func (pc *proxyClient) write() {
	log.Debugf("[proxyClient] %v begin write", pc.ID)
	defer pc.release()
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("recover from panic, err: %v", err)
		}
	}()

	for {
		select {
		case data := <-pc.writechan:
			if data.Type == protocol.ConstBlockTypeData {
				if err := pc.writeToLocal(data.Data); err != nil {
					return
				}
			}
		}
	}
}

func (pc *proxyClient) writeToLocal(b []byte) error {
	for {
		n, err := pc.conn.Write(b)
		if err != nil {
			log.Warnf("[proxyClient] write data to local failed, err: %v", err)
			return err
		}
		if n < len(b) {
			b = b[n:]
		} else {
			return nil
		}
	}
}
