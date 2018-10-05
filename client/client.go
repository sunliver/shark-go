package main

import (
	"flag"
	"fmt"
	"net"

	"github.com/pkg/profile"
	"github.com/sunliver/shark-go/client/localclient"

	log "github.com/sirupsen/logrus"
	client "github.com/sunliver/shark-go/client/localclient"
	"github.com/sunliver/shark-go/client/proxy"
)

type ProxyServer struct {
}

type ProxyServerConf struct {
	client.RemoteProxyConf
	Port     int
	Protocol string
}

func main() {
	if h {
		flag.Usage()
		return
	}
	log.SetLevel(log.Level(loglevel))

	if cpuprofile {
		defer profile.Start().Stop()
	}

	// addr = "localhost"
	// rp = 12306
	// lp = 10087
	// protocol = "http"
	RunServer(ProxyServerConf{
		RemoteProxyConf: client.RemoteProxyConf{
			// RemotePort:   8654,
			// RemoteServer: "shark.norgerman.com",
			RemotePort:   rp,
			RemoteServer: addr,
		},
		Port:     lp,
		Protocol: protocol,
	})
}

func RunServer(conf ProxyServerConf) error {
	m := localclient.NewManager(conf.RemoteProxyConf, coreSz)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Panicf("start server failed, err: %v", err)
		return err
	}
	log.Infof("local port: %v, remote: %v:%v", lp, addr, rp)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("listener get conn failed, err: %v", err)
			continue
		}

		c := m.GetClient()
		if err != nil {
			log.Errorf("connect to remote server failed, err: %v", err)
			conn.Close()
			continue
		}

		go proxy.ServerProxy(conf.Protocol, conn, c)
	}

	// TODO gracfully shutdown
	return nil
}
