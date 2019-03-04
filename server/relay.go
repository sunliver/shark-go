package server

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

type relay struct {
	id     uuid.UUID
	conn   net.Conn
	in     chan *block.BlockData
	log    logrus.FieldLogger
	ctx    context.Context
	cancel func()
	a      *Agent
}

const (
	constRelayBuf           = 16
	constRemoteReadBuf      = 4096
	constRemoteReadTimeout  = time.Second * 60
	constRemoteWriteTimeout = time.Second * 30
)

func newRelay(a *Agent, id uuid.UUID) *relay {
	c, cancel := context.WithCancel(a.ctx)
	return &relay{
		id:     id,
		ctx:    c,
		cancel: cancel,
		in:     make(chan *block.BlockData, constRelayBuf),
		log:    logrus.WithField("relay", short(id)),
		a:      a,
	}
}

type hostData struct {
	Address string `json:"Address"`
	Port    uint16 `json:"Port"`
}

func (r *relay) run() {
	defer r.release()

	r.log.Infof("run routine is start")
	defer func() {
		r.log.Infof("run routine is exit")
	}()

	for {
		select {
		case <-r.ctx.Done():
			err := r.ctx.Err()
			r.log.Infof("relay closed, %v", err)
			return
		case blockData, ok := <-r.in:
			if !ok {
				r.log.Infof("r.in is closed")
				return
			}

			r.log.Debugf("recv block, %sv", blockData)

			if blockData.Type == block.ConstBlockTypeConnect {
				var hosts hostData
				if err := json.Unmarshal(r.a.crypto.DecrypBlocks(blockData.Data), &hosts); err != nil {
					r.log.Errorf("broken connect block, %v", err)
					return
				}

				conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", hosts.Address, hosts.Port))
				if err != nil {
					r.log.Errorf("connect remote failed, %v", err)
					r.a.out <- block.Marshal(&block.BlockData{
						ID:   r.id,
						Type: block.ConstBlockTypeConnectFailed,
					})
					return
				}
				r.conn = conn
				r.a.out <- block.Marshal(&block.BlockData{
					ID:   r.id,
					Type: block.ConstBlockTypeConnected,
				})

				go r.write()
			} else {
				if r.conn == nil {
					// not connect remote yet
					// wrong sequence
					panic("conn is not init yet")
				}

				if err := r.conn.SetWriteDeadline(time.Now().Add(constRemoteWriteTimeout)); err != nil {
					r.log.Warnf("set remote write timeout failed, %v", err)
					return
				}

				if blockData.Type == block.ConstBlockTypeData {
					if blockData.Length > 0 {
						d := r.a.crypto.DecrypBlocks(blockData.Data)
						if n, err := r.conn.Write(d); err != nil || n < len(d) {
							r.log.Warnf("write to remote failed, %v", err)
							return
						}
					}
				} else {
					// simply drop the package
					r.log.Warnf("unrecognized block, %v", blockData)
				}
			}
		}
	}
}

func (r *relay) write() {
	defer r.release()
	r.log.Infof("write routine is start")
	defer func() {
		r.log.Infof("write routine is exit")
	}()

	var blockNum uint32
	for {
		if err := r.conn.SetReadDeadline(time.Now().Add(constRemoteReadTimeout)); err != nil {
			r.log.Warnf("set remote read timeout failed %v", err)
			return
		}

		select {
		case <-r.ctx.Done():
			r.log.Infof("write is canceled, %v", r.ctx.Err())
			return
		default:
			buf := make([]byte, constRemoteReadBuf)
			n, err := io.ReadAtLeast(r.conn, buf, 1)
			if err != nil {
				r.log.Warnf("read from remote failed, %v", err)
				return
			}

			r.a.out <- block.Marshal(&block.BlockData{
				ID:       r.id,
				BlockNum: blockNum,
				Type:     block.ConstBlockTypeData,
				Data:     r.a.crypto.CryptBlocks(buf[:n]),
			})
			blockNum++
		}
	}
}

func (r *relay) release() {
	r.a.unregisterRelay(r)
	r.cancel()

	r.log.Info("relay is released")
}
