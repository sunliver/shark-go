package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/sunliver/shark/lib/block"
	"github.com/sunliver/shark/lib/crypto"
)

const (
	constReadTimeoutS  = time.Second * 60
	constWriteTimeoutS = time.Second * 60
)

const (
	relayBusSz = 64
)

// relay struct
// connect with remote server
type relay struct {
	ID     uuid.UUID
	conn   net.Conn
	ctx    context.Context
	bus    chan []byte
	crypto *crypto.Crypto
	log    logrus.FieldLogger
	mutex  sync.Mutex
	agents map[uuid.UUID]*agent
	cancel func()
}

func newRelay(ctx context.Context, remote string) (*relay, error) {
	c, cancel := context.WithCancel(ctx)
	conn, err := net.Dial("tcp", remote)
	if err != nil {
		return nil, fmt.Errorf("init to remote server failed, err: %v", err)
	}

	id := uuid.NewV4()
	r := &relay{
		ID:     id,
		conn:   conn,
		ctx:    c,
		cancel: cancel,
		agents: make(map[uuid.UUID]*agent),
		bus:    make(chan []byte, relayBusSz),
		log:    logrus.WithField("relay", short(id)).WithField("conn", conn.RemoteAddr()),
	}

	if err := r.handshake(); err != nil {
		r.log.Errorf("handshake failed, %v", err)
		return r, err
	}

	go r.read()
	go r.write()

	return r, nil
}

// handshake do handshake with remote proxy server
func (c *relay) handshake() error {
	// step1: send syn
	{
		if _, err := c.conn.Write(block.Marshal(&block.BlockData{
			Type: block.ConstBlockTypeHandShake,
		})); err != nil {
			return err
		}

		buf := make([]byte, block.ConstBlockHeaderSzB)
		if n, err := io.ReadFull(c.conn, buf); err != nil || n < len(buf) {
			return err
		}

		if blockData, err := block.UnMarshalHeader(buf); err != nil || blockData.Type != block.ConstBlockTypeHandShake {
			return err
		}
	}

	passwd := []byte(block.NewGUID().String())

	// step2: send pass negotiation
	{
		if _, err := c.conn.Write(block.Marshal(&block.BlockData{
			ID:   block.NewGUID(),
			Type: block.ConstBlockTypeHandShakeResponse,
			Data: passwd,
		})); err != nil {
			return err
		}
	}

	// step3: recv handshake final
	{
		buf := make([]byte, block.ConstBlockHeaderSzB)

		if n, err := io.ReadFull(c.conn, buf); err != nil || n < len(buf) {
			return err
		}

		if blockData, err := block.UnMarshalHeader(buf); err != nil || blockData.Type != block.ConstBlockTypeHandShakeFinal {
			return err
		}
	}

	c.crypto = crypto.NewCrypto(passwd)

	return nil
}

func (c *relay) read() {
	c.log.Infof("read routine start")
	defer c.log.Infof("read routine stop")
	defer c.release()

	for {
		select {
		case <-c.ctx.Done():
			c.log.Infof("read recv done, %v", c.ctx.Err())
			return
		default:
			buf := make([]byte, block.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(c.conn, buf); err != nil || n < len(buf) {
				c.log.Warnf("read header failed, %v", err)
				return
			}

			blockData, err := block.UnMarshalHeader(buf)
			if err != nil {
				c.log.Errorf("broken header, %v", err)
				return
			}

			if blockData.Length > 0 {
				body := make([]byte, blockData.Length)
				if n, err := io.ReadFull(c.conn, body); err != nil || n < int(blockData.Length) {
					c.log.Warnf("read body failed, %v", err)
					return
				}

				blockData.Data = body
			}

			c.log.Debugf("recv block: %v", blockData)

			if ob, ok := c.agents[blockData.ID]; ok {
				// TODO add time out
				ob.bus <- blockData
			}
		}
	}
}

func (c *relay) write() {
	c.log.Infof("write routine start")
	defer c.log.Infof("write routine stop")
	defer c.release()

	for {
		select {
		case <-c.ctx.Done():
			c.log.Infof("write recv done, %v", c.ctx.Err())
			return
		case b, ok := <-c.bus:
			if !ok {
				c.log.Infof("write listen closed channel")
				return
			}
			if n, err := c.conn.Write(b); err != nil || n < len(b) {
				c.log.Warnf("write to remote failed, %v", err)
				return
			}
		}
	}
}

// registerAgent when receiving msgs, client will decode it and give to interested observers
func (c *relay) registerAgent(a *agent) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.agents[a.ID] = a
}

// unregisterAgent stop receive msg from client
func (c *relay) unregisterAgent(a *agent) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.agents, a.ID)
}

// release notify observers I'm out
func (c *relay) release() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cancel()
	c.conn.Close()

	c.log.Infof("relay is closed")
}
