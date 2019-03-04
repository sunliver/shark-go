package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/sunliver/shark/lib/block"
	"github.com/sunliver/shark/lib/crypto"
)

type Agent struct {
	ID     uuid.UUID
	conn   net.Conn
	crypto *crypto.Crypto
	log    logrus.FieldLogger
	out    chan []byte
	relays map[uuid.UUID]*relay
	mutex  sync.Mutex
	ctx    context.Context
	cancel func()
}

const (
	serverOutBuf   = 64
	serverRelayBuf = 64
)

func NewServer(ctx context.Context, conn net.Conn) *Agent {
	c, cancel := context.WithCancel(ctx)
	id := uuid.NewV4()
	return &Agent{
		ID:     id,
		ctx:    c,
		cancel: cancel,
		conn:   conn,
		relays: make(map[uuid.UUID]*relay, serverRelayBuf),
		out:    make(chan []byte, serverOutBuf),
		log:    logrus.WithField("agent", short(id)),
	}
}

func (a *Agent) Run() {
	defer a.release()

	a.log.Infof("run routine is start!")

	if err := a.handShake(); err != nil {
		a.log.Errorf("handshake failed, %v", err)
		return
	}

	a.log.Infof("handshake success, %v", a.conn.RemoteAddr())

	go a.writeBack()

	for {
		select {
		case <-a.ctx.Done():
			err := a.ctx.Err()
			a.log.Infof("close server, %v", err)
			return
		default:
			buf := make([]byte, block.ConstBlockHeaderSzB)
			if n, err := io.ReadFull(a.conn, buf); err != nil || n < len(buf) {
				a.log.Errorf("broken conn, %v", err)
				return
			}

			blockData, err := block.UnMarshalHeader(buf)
			if err != nil {
				a.log.Errorf("broken header, %v", err)
				return
			}

			if blockData.Type == block.ConstBlockTypeConnect {
				body := make([]byte, blockData.Length)
				if n, err := io.ReadFull(a.conn, body); err != nil || n < len(body) {
					a.log.Errorf("broken package, %v", err)
					return
				}

				blockData.Data = body
				a.registerRelay(newRelay(a, blockData.ID))
				go a.relays[blockData.ID].run()
				a.relays[blockData.ID].in <- blockData
			} else if blockData.Type == block.ConstBlockTypeData {
				body := make([]byte, blockData.Length)
				if n, err := io.ReadFull(a.conn, body); err != nil || n < len(body) {
					a.log.Errorf("broken package, %v", err)
					return
				}
				blockData.Data = body

				if relay, ok := a.relays[blockData.ID]; ok {
					relay.in <- blockData
				} else {
					a.log.Warn("send data to closed relay")
				}
			} else {
				a.log.Warnf("unrecognized block, %v", blockData)
			}
		}
	}
}

func (a *Agent) writeBack() {
	defer a.release()

	a.log.Infof("writeback routine is start!")

	for b := range a.out {
		if n, err := a.conn.Write(b); err != nil || n < len(b) {
			a.log.Warn("write back failed, %v", err)
			return
		}
	}
}

func (a *Agent) handShake() error {
	// 1. recv handshake and send handshake
	{
		buf := make([]byte, block.ConstBlockHeaderSzB)
		if n, err := io.ReadFull(a.conn, buf); err != nil || n < len(buf) {
			return err
		}

		blockData, err := block.UnMarshalHeader(buf)
		if err != nil {
			return err
		}
		if blockData.Type != block.ConstBlockTypeHandShake {
			return fmt.Errorf("expected handshake, get %v", blockData.Type)
		}

		handshakeData := block.Marshal(&block.BlockData{
			Type: block.ConstBlockTypeHandShake,
		})
		if n, err := a.conn.Write(handshakeData); err != nil || n < len(handshakeData) {
			return err
		}
	}

	// 2. recv passwd negotiation
	{
		buf := make([]byte, block.ConstBlockHeaderSzB)
		if n, err := io.ReadFull(a.conn, buf); err != nil || n < len(buf) {
			return err
		}

		blockData, err := block.UnMarshalHeader(buf)
		if err != nil {
			return err
		}
		if blockData.Type != block.ConstBlockTypeHandShakeResponse {
			return fmt.Errorf("expected handshake resp, get %v", blockData.Type)
		}

		if blockData.Length > 0 {
			body := make([]byte, blockData.Length)
			if n, err := io.ReadFull(a.conn, body); err != nil || n < len(body) {
				return err
			}

			a.crypto = crypto.NewCrypto(body)

			handShakeFinal := block.Marshal(&block.BlockData{
				ID:   blockData.ID,
				Type: block.ConstBlockTypeHandShakeFinal,
			})

			if n, err := a.conn.Write(handShakeFinal); err != nil || n < len(handShakeFinal) {
				return err
			}
		} else {
			return fmt.Errorf("invalid handshake resp")
		}
	}

	// ready to recv data
	return nil
}

func (a *Agent) registerRelay(r *relay) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.relays[r.id] = r

	a.log.Infof("relay is registered, %v", short(r.id))
}

func (a *Agent) unregisterRelay(r *relay) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	delete(a.relays, r.id)

	a.log.Infof("relay is unregistered, %v", short(r.id))
}

func (a *Agent) release() {
	a.cancel()
	a.conn.Close()

	a.log.Info("agent is closed")
}

func short(id uuid.UUID) string {
	return fmt.Sprintf("%x", id)[:8]
}
