package client

import (
	"container/list"
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager relay pool manager
// TODO add idle timeout and remove the relay
type Manager struct {
	ctx        context.Context
	clients    *list.List
	remote     string
	coreSz     int
	retryCnt   int
	retryDelay time.Duration
	mutex      sync.Mutex
	cancel     func()
	log        logrus.FieldLogger
}

// NewManager init relay pool manager with a fixed size
func NewManager(coreSz int, remote string) *Manager {
	c, cancel := context.WithCancel(context.Background())

	if coreSz < 0 {
		coreSz = runtime.NumCPU()
	}

	return &Manager{
		ctx:        c,
		clients:    list.New(),
		remote:     remote,
		coreSz:     coreSz,
		retryCnt:   5,
		retryDelay: time.Second * 1,
		cancel:     cancel,
		log:        logrus.WithField("manager", "1"),
	}
}

// Run accept a new conn with target proxy protocol
func (m *Manager) Run(conn net.Conn, protocol string) {
	c, err := m.getClient()
	if err != nil {
		conn.Close()
		return
	}

	a := newAgent(conn, protocol, c)
	a.run()
}

// Cancel cancel all hold relay
func (m *Manager) Cancel() {
	m.cancel()
}

// getClient return a relay which is ready to recv connections
func (m *Manager) getClient() (*relay, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// rm closed relay
	if m.clients.Len() > 0 {
		e := m.clients.Front()
		for {
			n := e.Next()
			r, ok := e.Value.(*relay)
			if !ok || r.closed {
				m.clients.Remove(e)
			}
			if n == nil {
				break
			}
			e = n
		}
	}

	// reinit relay pool
	if m.clients.Len() < m.coreSz {
		if err := m.initPool(); err != nil {
			return nil, err
		}
	}

	e := m.clients.Front()
	m.clients.PushBack(e)

	return e.Value.(*relay), nil
}

func (m *Manager) initPool() error {
	for m.clients.Len() < m.coreSz {
		i := 0
		for ; i < m.retryCnt; i++ {
			r, err := newRelay(m.ctx, m.remote)
			if err != nil {
				time.Sleep(m.retryDelay)
				m.log.Errorf("retry %v: init client failed, %v", i, err)
				continue
			}

			m.clients.PushBack(r)
			break
		}
		if i == m.retryCnt {
			return fmt.Errorf("init client failed too manay times")
		}
	}

	return nil
}
