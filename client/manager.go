package client

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager relay pool manager
type Manager struct {
	ctx    context.Context
	cancel func()
	slots  []*relay
	ticket *uint32
	mu     sync.Mutex
	remote string
	log    logrus.FieldLogger
}

const maxCoreSz = 100

// NewManager init relay pool manager with a fixed size
func NewManager(coreSz int, remote string) *Manager {
	c, cancel := context.WithCancel(context.Background())

	if coreSz < 0 {
		coreSz = runtime.NumCPU()
	}

	if coreSz > maxCoreSz {
		coreSz = maxCoreSz
	}

	return &Manager{
		ctx:    c,
		cancel: cancel,
		slots:  make([]*relay, coreSz),
		ticket: new(uint32),
		remote: remote,
		log:    logrus.WithField("manager", "1"),
	}
}

// Start accept a new conn with target Proxy protocol
func (m *Manager) Start(conn net.Conn, p Proxy) {
	c, err := m.getClient()
	if err != nil {
		_ = conn.Close()
		return
	}

	a := newAgent(conn, p, c)
	a.start()
}

// getClient return a relay which is ready to recv connections
func (m *Manager) getClient() (*relay, error) {
	// fast path: if current slot is ready, return it
	ticket := atomic.AddUint32(m.ticket, 1) - 1
	idx := ticket % uint32(len(m.slots))
	if r := m.slots[idx]; r != nil && r.closed != true {
		return r, nil
	}

	// slow path: create a new relay
	m.mu.Lock()
	defer m.mu.Unlock()
	// double check
	// other routine may create the relay
	if r := m.slots[idx]; r != nil && r.closed != true {
		return r, nil
	}

	retryCnt := 5
	retryDelay := time.Second * 1

	for i := 0; i < retryCnt; i++ {
		r, err := newRelay(m.ctx, m.remote)
		if err != nil {
			time.Sleep(retryDelay)
			m.log.Errorf("retry %v: init client failed, %v", i, err)
			continue
		}

		m.slots[idx] = r

		return r, nil
	}
	return nil, fmt.Errorf("connect with remote failed too many times")
}

// Cancel cancel all hold relay
func (m *Manager) Cancel() {
	m.cancel()
}
