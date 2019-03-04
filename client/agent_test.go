package client

import (
	"io"
	"net"
	"testing"

	"github.com/sunliver/shark/lib/block"

	log "github.com/sirupsen/logrus"
	"github.com/sunliver/shark/test/helper"

	"github.com/stretchr/testify/assert"
)

func TestAgentOK(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	cases := make(map[string]string)
	// cases["CONNECT www.sunliver.com:80 HTTP/1.1\r\nHello\r\n"] = "Hello\r\n"
	cases["GET http://www.sunliver.com/ HTTP/1.1\r\nHost: www.sunliver.com\r\nProxy-Connection: keep-alive\r\n\r\n"] = "GET http://www.sunliver.com/ HTTP/1.1\r\n" +
		"Host: www.sunliver.com\r\n" +
		"Proxy-Connection: keep-alive\r\n\r\n"

	for k, v := range cases {
		port := 9002
		uuid := block.NewGUID()
		go helper.NewEchoServer(t, port, uuid)

		conf := RemoteProxyConf{
			RemoteServer: "localhost",
			RemotePort:   port,
		}
		localclient, err := GetSingleClient(conf)
		if err != nil {
			t.Fatal("get localclient failed")
		}

		client, server := net.Pipe()
		defer func() {
			client.Close()
			server.Close()
		}()

		go func() {
			server.Write([]byte(k))
		}()

		pc := &agent{
			ID:        block.NewGUID(),
			proxy:     &httpProxy{},
			c:         localclient,
			conn:      client,
			writechan: make(chan *block.BlockData),
			ticket:    new(uint64),
			done:      new(uint64),
		}

		for pc.c.RegisterObserver(pc) != nil {
			pc.ID = block.NewGUID()
		}

		pc.start()

		// {
		// 	buf := make([]byte, len(constHTTPSuccess))
		// 	if n, err := io.ReadFull(server, buf); err != nil || n != len(buf) {
		// 		T.Fatal(err)
		// 	}
		// 	assert.Equal(T, constHTTPSuccess, string(buf))
		// }

		{
			buf := make([]byte, len(v))
			if n, err := io.ReadFull(server, buf); err != nil || n != len(buf) {
				t.Fatal(err)
			}
			assert.Equal(t, v, string(buf))
		}

		pc.release()
	}
}
