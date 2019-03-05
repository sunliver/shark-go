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

// agent handle connection from local
// send messages via relay
type agent struct {
	ID        uuid.UUID
	r         *relay
	conn      net.Conn
	proxy     proxy
	writechan chan *block.BlockData
	rls       sync.Once
	ticket    *uint64
	done      *uint64
}

func newAgent(conn net.Conn, p string, r *relay) *agent {
	return &agent{
		ID:        block.NewGUID(),
		proxy:     &httpProxy{},
		conn:      conn,
		r:         r,
		writechan: make(chan *block.BlockData),
		ticket:    new(uint64),
		done:      new(uint64),
	}
}

func (a *agent) run() {
	a.start()
}

// start agent start accept requests
func (a *agent) start() {
	blockData, remain, err := a.proxy.handShake(a.conn)
	if err != nil && err != errHTTPDegrade {
		log.Errorf("[agent] get proxy handshake msg failed, err: %v", err)
		a.release()
		return
	}

	log.Infof("[agent] send handshake msg, %v", string(blockData.Data))

	blockData.ID = a.ID
	if _, err := a.r.Write(blockData); err != nil {
		log.Warnf("[agent] send handshake msg failed, err: %v", err)
		a.release()
		return
	}

	// wait connected resp
	data := <-a.writechan

	if data.Type == block.ConstBlockTypeConnected {
		if a.proxy.T() == constProxyTypeHTTPS {
			if err := a.writeToLocal(a.proxy.handShakeResp()); err != nil {
				a.release()
				return
			}
			log.Infof("[agent] write handshake resp")
		}
	} else if data.Type == block.ConstBlockTypeConnectFailed {
		log.Infof("[agent] recv connect failed, %s", data)
		a.release()
		return
	} else {
		log.Errorf("[agent] unknown data, %v:%v:%v:%v", blockData.ID, blockData.Type, blockData.BlockNum, blockData.Length)
		a.release()
		return
	}

	// send http remain
	if _, err := a.r.Write(&block.BlockData{
		ID:   a.ID,
		Type: block.ConstBlockTypeData,
		Data: remain,
	}); err != nil {
		log.Errorf("[agent] send http msg failed, err: %v", err)
		a.release()
		return
	}

	go a.beginRead()
	go a.write()
}

func (a *agent) release() {
	a.rls.Do(func() {
		a.r.UnRegisterObserver(a)
		a.conn.Close()
	})

	log.Infof("[agent] %v released", a.ID)
}

func (a *agent) beginRead() {
	log.Debugf("[agent] %v begin read", a.ID)
	defer a.release()

	for {
		buf := make([]byte, 4096)
		n, err := io.ReadAtLeast(a.conn, buf, 1)
		if err != nil {
			log.Infof("[agent] read from local failed, err: %v", err)
			break
		}

		if _, err := a.r.Write(&block.BlockData{
			ID:   a.ID,
			Type: block.ConstBlockTypeData,
			Data: buf[:n],
		}); err != nil {
			log.Warnf("[agent] write data to remote failed, err: %v", err)
			break
		}
	}
}

func (a *agent) onRead(ticket uint64, data *block.BlockData, err error) {
	for !atomic.CompareAndSwapUint64(a.done, ticket, ticket) {
		runtime.Gosched()
	}

	if err != nil {
		log.Infof("[agent] client is closed, err: %v", err)
		a.release()
		return
	}

	a.writechan <- data
	atomic.AddUint64(a.done, 1)
}

func (a *agent) write() {
	log.Debugf("[agent] %v begin write", a.ID)
	defer a.release()
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("recover from panic, err: %v", err)
		}
	}()

	for {
		select {
		case data := <-a.writechan:
			if data.Type == block.ConstBlockTypeData {
				if err := a.writeToLocal(data.Data); err != nil {
					return
				}
			} else if data.Type == block.ConstBlockTypeDisconnect {
				log.Infof("remote closed")
				return
			}
		}
	}
}

func (a *agent) writeToLocal(b []byte) error {
	for {
		n, err := a.conn.Write(b)
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
