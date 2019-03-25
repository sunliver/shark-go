package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

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
	hostData, remain, err := a.proxy.HandShake(a.conn)
	if err != nil {
		a.log.Errorf("get proxy handshake msg failed, %v", err)
		a.release()
		return
	}

	a.log.Debugf("send handshake msg, %v", hostData)

	connectData, _ := json.Marshal(hostData)
	a.r.bus <- block.Marshal(&block.BlockData{
		ID:   a.ID,
		Type: block.ConstBlockTypeConnect,
		Data: a.r.crypto.CryptBlocks([]byte(connectData)),
	})

	defer a.release()

	// waiting for the first connected block
	select {
	case data := <-a.bus:
		if data.Type == block.ConstBlockTypeConnected {
			if a.proxy.GetProxyType() == proxyHTTPS {
				resp := a.proxy.HandShakeResp()
				if n, err := a.conn.Write(resp); err != nil || n < len(resp) {
					a.log.Warnf("write back failed, %v", err)
					return
				}
			}
		} else if data.Type == block.ConstBlockTypeConnectFailed {
			a.log.Warnf("recv connect failed")
			return
		} else {
			a.log.Warnf("unrecognized block data, %v", hostData)
			return
		}

		if remain != nil && len(remain) > 0 {
			a.r.bus <- block.Marshal(&block.BlockData{
				ID:   a.ID,
				Type: block.ConstBlockTypeData,
				Data: a.r.crypto.CryptBlocks(remain),
			})
		}
	case <-time.After(time.Second * 30):
		a.log.Errorf("wait connected block timeout")
		return
	}

	// begin read from remote, then
	// write to local
	go a.write()

	// read func is inside run func, then
	// run func can simply use `defer a.release()` to cleanup

	// begin read from local, then
	// write to remote
	a.log.Debugf("read routine start")
	defer a.log.Debugf("read routine stop")
	for {
		select {
		case <-a.ctx.Done():
			a.log.Infof("read recv done, %v", a.ctx.Err())
			return
		default:
			buf := make([]byte, 4096)
			n, err := io.ReadAtLeast(a.conn, buf, 1)
			if err != nil {
				a.log.Warnf("read from local failed, err: %v", err)
				return
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
	a.log.Debugf("write routine start")
	defer a.log.Debugf("write routine stop")
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
				d := a.r.crypto.DecrypBlocks(data.Data)
				if n, err := a.conn.Write(d); err != nil || n < len(d) {
					a.log.Warnf("write back failed, %v", err)
					return
				}
			} else if data.Type == block.ConstBlockTypeDisconnect {
				a.log.Infof("remote closed")
				return
			} else {
				a.log.Warnf("unrecognized block type")
				return
			}
		}
	}
}

func (a *agent) release() {
	a.cancel()
	a.r.unregisterAgent(a)
	_ = a.conn.Close()

	a.log.Debugf("agent is closed")
}

func short(id uuid.UUID) string {
	return fmt.Sprintf("%x", id)[:8]
}
