package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/sunliver/shark/protocol"
	"github.com/sunliver/shark/utils"
)

const (
	constReadTimeoutS  = time.Second * 60
	constWriteTimeoutS = time.Second * 60
)

const (
	constStatusHandShaking = iota
	constStatusIdle
	constStatusRunning
	constStatusClosed
)

// relay struct
// connect with remote server
type relay struct {
	ID      uuid.UUID
	conn    *net.TCPConn
	Status  int
	Lasterr error
	mutex   sync.Mutex
	readOBs map[uuid.UUID]*agent
	crypto  *utils.Crypto
	rls     sync.Once
}

// RemoteProxyConf configuration of remote server
type RemoteProxyConf struct {
	RemoteServer string
	RemotePort   int
}

func (conf *RemoteProxyConf) String() string {
	return fmt.Sprintf("%v:%v", conf.RemoteServer, conf.RemotePort)
}

var errClose = errors.New("relay: closed")

// initClient returns client to connect proxy server;
// tcp connection
func initClient(conf RemoteProxyConf) (*relay, error) {
	c := &relay{
		ID: uuid.NewV4(),
	}
	c.readOBs = make(map[uuid.UUID]*agent)

	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", conf.RemoteServer, conf.RemotePort))
	if err != nil {
		c.Lasterr = fmt.Errorf("resolve remote addr failed, err: %v", err)
		return c, c.Lasterr
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		c.Lasterr = fmt.Errorf("init to remote server failed, err: %v", err)
		return c, c.Lasterr
	}

	c.conn = conn

	if err := c.conn.SetKeepAlive(true); err != nil {
		c.Lasterr = fmt.Errorf("set keep alive failed, err: %v", err)
		return c, c.Lasterr
	}

	c.Status = constStatusHandShaking

	// TODO client lifecycle
	if err := c.handshake(); err != nil {
		c.Lasterr = err
		return c, c.Lasterr
	}

	c.Lasterr = nil
	c.Status = constStatusRunning
	go c.beginRead()

	return c, c.Lasterr
}

// handshake do handshake with remote proxy server
// deal with ConstBlockTypeHandShake, ConstBlockTypeHandShakeResponse, ConstBlockTypeHandShakeFinal
func (c *relay) handshake() error {
	if _, err := c.conn.Write(protocol.Marshal(&protocol.BlockData{
		Type: protocol.ConstBlockTypeHandShake,
	})); err != nil {
		log.Errorf("[relay %v] handshake with remote server faild, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	buf := make([]byte, protocol.ConstBlockHeaderSzB)

	if n, err := io.ReadFull(c.conn, buf); err != nil || n < protocol.ConstBlockHeaderSzB {
		log.Errorf("[relay %v] read handshake failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if blockdata, err := protocol.UnMarshalHeader(buf); err != nil || blockdata.Type != protocol.ConstBlockTypeHandShake {
		log.Errorf("[relay %v] error handshake msg", c.ID)
		c.Lasterr = errors.New("err handshake msg")
		return c.Lasterr
	}

	passwd := []byte(protocol.NewGUID().String())

	if _, err := c.conn.Write(protocol.Marshal(&protocol.BlockData{
		ID:   protocol.NewGUID(),
		Type: protocol.ConstBlockTypeHandShakeResponse,
		Data: passwd,
	})); err != nil {
		log.Errorf("[relay %v] send handshake resp failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if n, err := io.ReadFull(c.conn, buf); err != nil || n < protocol.ConstBlockHeaderSzB {
		log.Errorf("[relay %v] read handshake resp failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if blockdata, err := protocol.UnMarshalHeader(buf); err != nil || blockdata.Type != protocol.ConstBlockTypeHandShakeFinal {
		log.Errorf("[relay %v] err handshake final msg", c.ID)
		c.Lasterr = errors.New("err handshake final msg")
		return c.Lasterr
	}

	c.crypto = utils.NewCrypto(passwd)

	return nil
}

func (c *relay) beginRead() {
	defer c.release()

	for {
		header := make([]byte, protocol.ConstBlockHeaderSzB)
		if n, err := io.ReadFull(c.conn, header); err != nil || n < protocol.ConstBlockHeaderSzB {
			log.Warnf("[relay %v] read header failed, err: %v", c.ID, err)
			c.Lasterr = err
			return
		}

		blockData, err := protocol.UnMarshalHeader(header)
		if err != nil {
			log.Errorf("[relay %v] unmarshal datablock failed, err: %v", c.ID, err)
			c.Lasterr = err
			// drop the connection
			return
		}

		log.Debugf("[relay %v] recv %s", c.ID, blockData)

		if blockData.Length > 0 {
			body := make([]byte, blockData.Length)
			if n, err := io.ReadFull(c.conn, body); err != nil || n < int(blockData.Length) {
				log.Errorf("[relay %v] read body failed, err: %v", c.ID, err)
				c.Lasterr = err
				return
			}

			blockData.Data = c.crypto.DecrypBlocks(body)
			blockData.Length = int32(len(blockData.Data))
		}

		if blockData.Type == protocol.ConstBlockTypeDisconnect {
			var ids []string
			if err := json.Unmarshal(blockData.Data, &ids); err != nil {
				log.Errorf("[relay %v] recv bad disconnect data %v", c.ID, string(blockData.Data))
				c.Lasterr = err
				return
			}

			for _, id := range ids {
				uid, err := uuid.FromString(id)
				if err != nil {
					log.Errorf("[relay %v] unmarshal uuid failed, err: %v, uuid: %v", c.ID, err, id)
					c.Lasterr = err
					return
				}

				if ob, ok := c.readOBs[uid]; ok {
					go ob.onRead(atomic.AddUint64(ob.ticket, 1)-1, nil, errClose)
				}
			}
		}

		if ob, ok := c.readOBs[blockData.ID]; ok {
			go ob.onRead(atomic.AddUint64(ob.ticket, 1)-1, blockData, nil)
		}
	}
}

func (c *relay) Write(blockdata *protocol.BlockData) (int, error) {
	if c.Status != constStatusRunning {
		return 0, fmt.Errorf("[relay %v] client is not running", c.ID)
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(constWriteTimeoutS)); err != nil {
		c.release()
		return 0, err
	}

	if len(blockdata.Data) > 0 {
		blockdata.Data = c.crypto.CryptBlocks(blockdata.Data)
	}
	b := protocol.Marshal(blockdata)
	log.Debugf("[relay %v] send %s", c.ID, blockdata)
	return c.conn.Write(b)
}

// RegisterObserver when receiving msgs, client will decode it and give to interested observers
func (c *relay) RegisterObserver(a *agent) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, ok := c.readOBs[a.ID]; ok {
		return fmt.Errorf("[relay %v] duplicate observer, %v", c.ID, a)
	}

	c.readOBs[a.ID] = a
	return nil
}

// UnRegisterObserver stop receive msg from client
func (c *relay) UnRegisterObserver(a *agent) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.readOBs, a.ID)
}

// release notify observers I'm out
func (c *relay) release() {
	c.rls.Do(func() {
		c.Lasterr = errClose
		for id, ob := range c.readOBs {
			log.Debugf("[relay %v] %v i'm closing", c.ID, id)
			ob.onRead(atomic.AddUint64(ob.ticket, 1)-1, nil, errClose)
		}
		c.readOBs = nil
		c.conn.Close()
		c.Status = constStatusClosed
	})

	log.Debugf("[relay %v] client is released", c.ID)
}
