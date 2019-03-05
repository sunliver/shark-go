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
	a      *Agent
	bus    chan *block.BlockData
	log    logrus.FieldLogger
	ctx    context.Context
	cancel func()
}

const (
	relayBusSz              = 16
	remoteReadBufSz         = 4096
	constRemoteReadTimeout  = time.Second * 60
	constRemoteWriteTimeout = time.Second * 60
)

func newRelay(a *Agent, id uuid.UUID) *relay {
	c, cancel := context.WithCancel(a.ctx)
	return &relay{
		id:     id,
		ctx:    c,
		cancel: cancel,
		bus:    make(chan *block.BlockData, relayBusSz),
		log:    logrus.WithField("relay", short(id)),
		a:      a,
	}
}

func (r *relay) run() {
	r.log.Infof("run routine start")
	defer r.log.Infof("run routine stop")
	defer r.release()

	for {
		select {
		case <-r.ctx.Done():
			err := r.ctx.Err()
			r.log.Infof("relay closed, %v", err)
			return
		case blockData, ok := <-r.bus:
			if !ok {
				r.log.Infof("r.bus is closed")
				return
			}

			r.log.Debugf("recv block, %v", blockData)

			if blockData.Type == block.ConstBlockTypeConnect {
				var hosts block.HostData
				if len(blockData.Data) > 0 {
					if err := json.Unmarshal(r.a.crypto.DecrypBlocks(blockData.Data), &hosts); err != nil {
						r.log.Errorf("broken connect block, %v", err)
						return
					}
				} else {
					r.log.Errorf("broken connect block, without hostdata")
					return
				}

				conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", hosts.Address, hosts.Port))
				if err != nil {
					r.log.Errorf("connect remote failed, %v", err)
					r.a.bus <- block.Marshal(&block.BlockData{
						ID:   r.id,
						Type: block.ConstBlockTypeConnectFailed,
					})
					return
				}
				r.conn = conn
				r.log = r.log.WithField("conn", r.conn.RemoteAddr())
				r.a.bus <- block.Marshal(&block.BlockData{
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
	r.log.Debugf("write routine start")
	defer r.log.Debugf("write routine stop")
	defer r.release()

	var blockNum uint32
	for {
		select {
		case <-r.ctx.Done():
			r.log.Infof("write is canceled, %v", r.ctx.Err())
			return
		default:
			buf := make([]byte, remoteReadBufSz)
			n, err := io.ReadAtLeast(r.conn, buf, 1)
			if err != nil {
				r.log.Warnf("read from remote failed, %v", err)

				r.a.bus <- block.Marshal(&block.BlockData{
					ID:       r.id,
					BlockNum: blockNum,
					Type:     block.ConstBlockTypeDisconnect,
				})
				return
			}

			r.a.bus <- block.Marshal(&block.BlockData{
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

	r.log.Infof("relay is released")
}
