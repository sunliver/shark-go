package client

import (
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/sunliver/shark/lib/block"
)

// agent handle connection from local
// send messages via relay
type agent struct {
	ID        uuid.UUID
	c         *relay
	conn      net.Conn
	proxy     proxy
	writechan chan *block.BlockData
	rls       sync.Once
	ticket    *uint64
	done      *uint64
}

const (
	constProxyTypeHTTP = iota
	constProxyTypeHTTPS
	constProxyTypeSocks4
	constProxyTypeSocks5
)

type proxy interface {
	// HandShake returns proxy handshake msg
	handShake(conn net.Conn) (blockdata *block.BlockData, read []byte, err error)
	// HandShakeResp returns proxy handshake resp msg
	handShakeResp() []byte
	T() int
}

type hostData struct {
	Address string `json:"Address"`
	Port    uint16 `json:"Port"`
}

// ServerProxy handle proxy connections
func ServerProxy(p string, conn net.Conn, c *relay) {
	p = "http"
	pc := &agent{
		ID:        block.NewGUID(),
		proxy:     &httpProxy{},
		c:         c,
		conn:      conn,
		writechan: make(chan *block.BlockData),
		ticket:    new(uint64),
		done:      new(uint64),
	}

	for pc.c.RegisterObserver(pc) != nil {
		pc.ID = block.NewGUID()
	}
	pc.start()
}

// start agent start accept requests
func (pc *agent) start() {
	blockdata, remain, err := pc.proxy.handShake(pc.conn)
	if err != nil && err != errHTTPDegrade {
		log.Errorf("[agent] get proxy handshake msg failed, err: %v", err)
		pc.release()
		return
	}

	log.Infof("[agent] send handshake msg, %v", string(blockdata.Data))

	blockdata.ID = pc.ID
	if _, err := pc.c.Write(blockdata); err != nil {
		log.Warnf("[agent] send handshake msg failed, err: %v", err)
		pc.release()
		return
	}

	// wait connected resp
	data := <-pc.writechan

	if data.Type == block.ConstBlockTypeConnected {
		if pc.proxy.T() == constProxyTypeHTTPS {
			if err := pc.writeToLocal(pc.proxy.handShakeResp()); err != nil {
				pc.release()
				return
			}
			log.Infof("[agent] write handshake resp")
		}
	} else if data.Type == block.ConstBlockTypeConnectFailed {
		log.Infof("[agent] recv connect failed, %s", data)
		pc.release()
		return
	} else {
		log.Errorf("[agent] unknown data, %v:%v:%v:%v", blockdata.ID, blockdata.Type, blockdata.BlockNum, blockdata.Length)
		pc.release()
		return
	}

	// send http remain
	if _, err := pc.c.Write(&block.BlockData{
		ID:   pc.ID,
		Type: block.ConstBlockTypeData,
		Data: remain,
	}); err != nil {
		log.Errorf("[agent] send http msg failed, err: %v", err)
		pc.release()
		return
	}

	go pc.beginRead()
	go pc.write()
}

func (pc *agent) release() {
	pc.rls.Do(func() {
		pc.c.UnRegisterObserver(pc)
		pc.conn.Close()
	})

	log.Infof("[agent] %v released", pc.ID)
}

func (pc *agent) beginRead() {
	log.Debugf("[agent] %v begin read", pc.ID)
	defer pc.release()

	for {
		buf := make([]byte, 4096)
		n, err := io.ReadAtLeast(pc.conn, buf, 1)
		if err != nil {
			log.Infof("[agent] read from local failed, err: %v", err)
			break
		}

		if _, err := pc.c.Write(&block.BlockData{
			ID:   pc.ID,
			Type: block.ConstBlockTypeData,
			Data: buf[:n],
		}); err != nil {
			log.Warnf("[agent] write data to remote failed, err: %v", err)
			break
		}
	}
}

func (pc *agent) onRead(ticket uint64, data *block.BlockData, err error) {
	for !atomic.CompareAndSwapUint64(pc.done, ticket, ticket) {
		runtime.Gosched()
	}

	if err != nil {
		log.Infof("[agent] client is closed, err: %v", err)
		pc.release()
		return
	}

	log.Debugf("agent: %v, ticket: %v\n", pc.ID, ticket)

	pc.writechan <- data

	atomic.AddUint64(pc.done, 1)
}

func (pc *agent) write() {
	log.Debugf("[agent] %v begin write", pc.ID)
	defer pc.release()
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("recover from panic, err: %v", err)
		}
	}()

	for {
		select {
		case data := <-pc.writechan:
			if data.Type == block.ConstBlockTypeData {
				if err := pc.writeToLocal(data.Data); err != nil {
					return
				}
			}
		}
	}
}

func (pc *agent) writeToLocal(b []byte) error {
	for {
		n, err := pc.conn.Write(b)
		if err != nil {
			log.Warnf("[agent] write data to local failed, err: %v", err)
			return err
		}
		if n < len(b) {
			b = b[n:]
		} else {
			return nil
		}
	}
}
