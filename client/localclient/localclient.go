package localclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/sunliver/shark-go/protocol"
	"github.com/sunliver/shark-go/utils"
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

// Client struct of local client
// connect with remote server
type Client struct {
	ID      uuid.UUID
	conn    *net.TCPConn
	readOBs map[uuid.UUID]func(data *protocol.BlockData, err error)
	Status  int
	Lasterr error
	mutex   sync.Mutex
	crypto  *utils.Crypto
	rlsOnce sync.Once
}

// RemoteProxyConf configuration of remote server
type RemoteProxyConf struct {
	RemoteServer string
	RemotePort   int
}

func (conf *RemoteProxyConf) String() string {
	return fmt.Sprintf("%v:%v", conf.RemoteServer, conf.RemotePort)
}

var errClose = errors.New("localclient: closed")

// initClient returns client to connect proxy server;
// tcp connection
func initClient(conf RemoteProxyConf) (*Client, error) {
	c := &Client{
		ID: uuid.NewV4(),
	}
	c.readOBs = make(map[uuid.UUID]func(data *protocol.BlockData, err error))

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
func (c *Client) handshake() error {
	if _, err := c.conn.Write(protocol.Marshal(&protocol.BlockData{
		Type: protocol.ConstBlockTypeHandShake,
	})); err != nil {
		log.Errorf("[Client %v] handshake with remote server faild, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	buf := make([]byte, protocol.ConstBlockHeaderSzB)

	if n, err := io.ReadFull(c.conn, buf); err != nil || n < protocol.ConstBlockHeaderSzB {
		log.Errorf("[Client %v] read handshake failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if blockdata, err := protocol.UnMarshalHeader(buf); err != nil || blockdata.Type != protocol.ConstBlockTypeHandShake {
		log.Errorf("[Client %v] error handshake msg", c.ID)
		c.Lasterr = errors.New("err handshake msg")
		return c.Lasterr
	}

	passwd := []byte(protocol.NewGUID().String())

	if _, err := c.conn.Write(protocol.Marshal(&protocol.BlockData{
		ID:   protocol.NewGUID(),
		Type: protocol.ConstBlockTypeHandShakeResponse,
		Data: passwd,
	})); err != nil {
		log.Errorf("[Client %v] send handshake resp failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if n, err := io.ReadFull(c.conn, buf); err != nil || n < protocol.ConstBlockHeaderSzB {
		log.Errorf("[Client %v] read handshake resp failed, err: %v", c.ID, err)
		c.Lasterr = err
		return c.Lasterr
	}

	if blockdata, err := protocol.UnMarshalHeader(buf); err != nil || blockdata.Type != protocol.ConstBlockTypeHandShakeFinal {
		log.Errorf("[Client %v] err handshake final msg", c.ID)
		c.Lasterr = errors.New("err handshake final msg")
		return c.Lasterr
	}

	c.crypto = utils.NewCrypto(passwd)

	return nil
}

func (c *Client) beginRead() {
	defer c.release()

	for {
		header := make([]byte, protocol.ConstBlockHeaderSzB)
		if n, err := io.ReadFull(c.conn, header); err != nil || n < protocol.ConstBlockHeaderSzB {
			log.Warnf("[Client %v] read header failed, err: %v", c.ID, err)
			c.Lasterr = err
			return
		}

		blockData, err := protocol.UnMarshalHeader(header)
		if err != nil {
			log.Errorf("[Client %v] unmarshal datablock failed, err: %v", c.ID, err)
			c.Lasterr = err
			// drop the connection
			return
		}

		log.Debugf("[Client %v] recv %s", c.ID, blockData)

		if blockData.Length > 0 {
			body := make([]byte, blockData.Length)
			if n, err := io.ReadFull(c.conn, body); err != nil || n < int(blockData.Length) {
				log.Errorf("[Client %v] read body failed, err: %v", c.ID, err)
				c.Lasterr = err
				return
			}

			blockData.Data = c.crypto.DecrypBlocks(body)
			blockData.Length = int32(len(blockData.Data))
		}

		if blockData.Type == protocol.ConstBlockTypeDisconnect {
			var ids []string
			if err := json.Unmarshal(blockData.Data, &ids); err != nil {
				log.Errorf("[Client %v] recv bad disconnect data %v", c.ID, string(blockData.Data))
				c.Lasterr = err
				return
			}

			for _, id := range ids {
				uid, err := uuid.FromString(id)
				if err != nil {
					log.Errorf("[Client %v] unmarshal uuid failed, err: %v, uuid: %v", c.ID, err, id)
					c.Lasterr = err
					return
				}

				if ob, ok := c.readOBs[uid]; ok {
					ob(nil, errClose)
				}
			}
		}

		if ob, ok := c.readOBs[blockData.ID]; ok {
			ob(blockData, nil)
		}
	}
}

func (c *Client) Write(blockdata *protocol.BlockData) (int, error) {
	if c.Status != constStatusRunning {
		return 0, fmt.Errorf("[Client %v] client is not running", c.ID)
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(constWriteTimeoutS)); err != nil {
		c.release()
		return 0, err
	}

	if len(blockdata.Data) > 0 {
		blockdata.Data = c.crypto.CryptBlocks(blockdata.Data)
	}
	b := protocol.Marshal(blockdata)
	log.Debugf("[Client %v] send %s", c.ID, blockdata)
	return c.conn.Write(b)
}

// RegisterObserver when receiving msgs, client will decode it and give to interested observers
func (c *Client) RegisterObserver(uuid uuid.UUID, f func(data *protocol.BlockData, err error)) error {
	if _, ok := c.readOBs[uuid]; ok {
		return fmt.Errorf("[Client %v] duplicate observer, %v", c.ID, uuid)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.readOBs[uuid] = f
	return nil
}

// UnRegisterObserver stop receive msg from client
func (c *Client) UnRegisterObserver(uuid uuid.UUID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.readOBs, uuid)
}

// release notify observers I'm out
func (c *Client) release() {
	c.rlsOnce.Do(func() {
		c.Lasterr = errClose
		for id, ob := range c.readOBs {
			log.Debugf("[Client %v] %v i'm closing", c.ID, id)
			ob(nil, errClose)
		}
		c.readOBs = nil
		c.conn.Close()
		c.Status = constStatusClosed
	})
	if c.Status != constStatusClosed {

	}
	log.Debugf("[Client %v] client is released", c.ID)
}
