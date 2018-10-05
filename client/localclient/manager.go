package localclient

import (
	"container/list"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Manager struct {
	clients    *list.List
	coreSz     int
	mutex      *sync.RWMutex
	conf       RemoteProxyConf
	retryCnt   int
	retryDelay time.Duration
}

func NewManager(conf RemoteProxyConf, coreSz int) *Manager {
	m := &Manager{
		clients:    list.New(),
		mutex:      &sync.RWMutex{},
		coreSz:     coreSz,
		conf:       conf,
		retryCnt:   10,
		retryDelay: time.Second * 1,
	}

	go m.initPool()

	return m
}

func GetSingleClient(conf RemoteProxyConf) (*Client, error) {
	return initClient(conf)
}

// GetClient return a localclient which is ready to recv connections
func (m *Manager) GetClient() *Client {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var c *Client

	for m.clients.Len() > 0 {
		e := m.clients.Front()
		if e.Value == nil || e.Value.(*Client).Status == constStatusClosed {
			m.clients.Remove(e)
			continue
		} else {
			m.clients.MoveToBack(e)
			c = e.Value.(*Client)
			break
		}
	}

	if m.clients.Len() == 0 || m.coreSz == -1 {
		for i := 0; i < m.retryCnt; i++ {
			cc, err := initClient(m.conf)
			if err != nil {
				panic(err)
			}

			if m.coreSz != -1 {
				m.clients.PushBack(cc)
			}
			c = cc
			break
		}
	}

	if m.clients.Len() < m.coreSz {
		go m.initPool()
	}

	return c
}

func (m *Manager) initPool() {
	if m.coreSz == -1 {
		return
	}

	for m.clients.Len() < m.coreSz {
		i := 0
		for ; i < m.retryCnt; i++ {
			c, err := initClient(m.conf)
			if err != nil {
				log.Errorf("[Manager] init client failed, retrying %v", i)
				time.Sleep(m.retryDelay * time.Duration(2<<uint32(i)))
			}

			m.mutex.Lock()
			m.clients.PushBack(c)
			m.mutex.Unlock()
			break
		}
		if i == m.retryCnt {
			panic("error connecting remote server")
		}
	}
}
