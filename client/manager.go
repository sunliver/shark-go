package client

import (
	"container/list"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Manager struct {
	clients    *list.List
	coreSz     int
	mutex      sync.RWMutex
	remote     string
	retryCnt   int
	retryDelay time.Duration
}

func NewManager(remote string, coreSz int) *Manager {
	m := &Manager{
		clients:    list.New(),
		coreSz:     coreSz,
		remote:     remote,
		retryCnt:   10,
		retryDelay: time.Second * 1,
	}

	// TODO lazy loading
	go m.initPool()

	return m
}

func (m *Manager) Run(conn net.Conn, p string) {
	c, err := m.getClient()
	if err != nil {
		// TODO
		// log.Errorf("connect with remote failed, %v", err)
		conn.Close()
		return
	}

	a := newAgent(conn, p, c)

	go a.run()
}

// GetClient return a localclient which is ready to recv connections
func (m *Manager) getClient() (*relay, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var c *relay

	for m.clients.Len() > 0 {
		e := m.clients.Front()
		if e.Value == nil {
			m.clients.Remove(e)
			continue
		} else {
			m.clients.MoveToBack(e)
			c = e.Value.(*relay)
			break
		}
	}

	if m.clients.Len() == 0 || m.coreSz == -1 {
		var e error
		for i := 0; i < m.retryCnt; i++ {
			cc, err := newRelay(m.remote)
			if err != nil {
				e = err
				continue
			}
			if m.coreSz != -1 {
				m.clients.PushBack(cc)
			}
			c = cc
			e = nil
			break
		}
		if e != nil {
			return nil, e
		}
	}

	if m.clients.Len() < m.coreSz {
		go m.initPool()
	}

	return c, nil
}

func (m *Manager) initPool() {
	if m.coreSz == -1 {
		return
	}

	for m.clients.Len() < m.coreSz {
		i := 0
		for ; i < m.retryCnt; i++ {
			c, err := newRelay(m.remote)
			if err != nil {
				log.Errorf("[Manager] init client failed, %v, retrying %v", err, i)
				time.Sleep(m.retryDelay * time.Duration(2<<uint32(i)))
				continue
			}

			m.mutex.Lock()
			m.clients.PushBack(c)
			m.mutex.Unlock()
			break
		}
		if i == m.retryCnt {
			log.Errorf("connect remote failed")
			return
		}
	}
}
