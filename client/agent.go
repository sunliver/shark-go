package client

import (
	"context"
	"fmt"
	"io"
	"net"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/sunliver/shark/lib/block"
)

const (
	agentBusSz = 64
)

// agent handle connection from local
type agent struct {
	ID     uuid.UUID
	conn   net.Conn
	r      *relay
	ctx    context.Context
	cancel func()
	proxy  proxy
	log    logrus.FieldLogger
	bus    chan *block.BlockData
}

func newAgent(conn net.Conn, p string, r *relay) *agent {
	c, cancel := context.WithCancel(r.ctx)
	id := block.NewGUID()
	a := &agent{
		ID:     id,
		proxy:  &httpProxy{},
		conn:   conn,
		ctx:    c,
		cancel: cancel,
		r:      r,
		bus:    make(chan *block.BlockData, agentBusSz),
		log:    logrus.WithField("agent", short(id)).WithField("conn", conn.RemoteAddr()),
	}
	a.r.registerAgent(a)
	return a
}

func (a *agent) run() {
	blockData, remain, err := a.proxy.handShake(a.conn)
	if err != nil && err != errHTTPDegrade {
		a.log.Errorf("get proxy handshake msg failed, %v", err)
		a.release()
		return
	}

	a.log.Debugf("send handshake msg, %v", string(blockData.Data))

	blockData.ID = a.ID
	if len(blockData.Data) > 0 {
		blockData.Data = a.r.crypto.CryptBlocks(blockData.Data)
	}
	a.r.bus <- block.Marshal(blockData)

	// wait connected resp
	data := <-a.bus

	if data.Type == block.ConstBlockTypeConnected {
		if a.proxy.T() == proxyTypeHTTPS {
			resp := a.proxy.handShakeResp()
			if n, err := a.conn.Write(resp); err != nil || n < len(resp) {
				a.log.Warnf("write back failed, %v", err)
				a.release()
				return
			}
		}
	} else if data.Type == block.ConstBlockTypeConnectFailed {
		a.log.Errorf("recv connect failed")
		a.release()
		return
	} else {
		a.log.Warnf("unrecognized block data, %v", blockData)
		a.release()
		return
	}

	a.r.bus <- block.Marshal(&block.BlockData{
		ID:   a.ID,
		Type: block.ConstBlockTypeData,
		Data: a.r.crypto.CryptBlocks(remain),
	})

	go a.read()
	go a.write()
}

func (a *agent) read() {
	a.log.Infof("read routine start")
	defer a.log.Infof("read routine stop")
	defer a.release()

	for {
		select {
		case <-a.ctx.Done():
			a.log.Infof("read recv done, %v", a.ctx.Err())
			return
		default:
			buf := make([]byte, 4096)
			n, err := io.ReadAtLeast(a.conn, buf, 1)
			if err != nil {
				a.log.Warnf("[agent] read from local failed, err: %v", err)
				break
			}

			a.r.bus <- block.Marshal(&block.BlockData{
				ID:   a.ID,
				Type: block.ConstBlockTypeData,
				Data: a.r.crypto.CryptBlocks(buf[:n]),
			})
		}
	}
}

func (a *agent) write() {
	a.log.Infof("write routine start")
	defer a.log.Infof("write routine stop")
	defer a.release()

	for {
		select {
		case <-a.ctx.Done():
			a.log.Infof("write recv done, %v", a.ctx.Err())
			return
		case data, ok := <-a.bus:
			if !ok {
				a.log.Infof("write listen closed channel")
				return
			}

			if data.Type == block.ConstBlockTypeData {
				if n, err := a.conn.Write(a.r.crypto.DecrypBlocks(data.Data)); err != nil || n < len(data.Data) {
					a.log.Warnf("write back failed, %v", err)
					return
				}
			} else if data.Type == block.ConstBlockTypeDisconnect {
				a.log.Infof("remote closed")
				return
			} else {
				a.log.Warnf("unrecognized block type")
			}
		}
	}
}

func (a *agent) release() {
	a.cancel()
	a.r.unregisterAgent(a)
	a.conn.Close()

	a.log.Infof("agent is closed")
}

func short(id uuid.UUID) string {
	return fmt.Sprintf("%x", id)[:8]
}
