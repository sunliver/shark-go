package client

import (
	"encoding/json"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/sunliver/shark/lib/block"
)

func initConn(msg string) net.Conn {
	client, server := net.Pipe()

	go func() {
		server.Write([]byte(msg))
		server.Close()
	}()

	return client
}

func TestHTTPHandShakeOK(t *testing.T) {
	httpstr := "GET http://www.sunliver.com/ HTTP/1.1\r\n" +
		"Host: www.sunliver.com\r\n" +
		"Proxy-Connection: keep-alive\r\n\r\n"
	client := initConn(httpstr)
	defer client.Close()

	proxy := &httpProxy{}
	blockdata, remain, err := proxy.handShake(client)
	hostdata, _ := json.Marshal(hostData{
		Address: "www.sunliver.com",
		Port:    uint16(80),
	})

	assert.Equal(t, errHTTPDegrade, err)
	assert.Equal(t, block.ConstBlockTypeConnect, blockdata.Type)
	assert.Equal(t, []byte(hostdata), blockdata.Data)
	assert.Equal(t, httpstr, string(remain))
}

func TestHTTPSHandShakeOK(t *testing.T) {
	httpsstr := "CONNECT www.sunliver.com:80 HTTP/1.1\r\nHost: www.sunliver.com\r\n\r\nHello\r\n"
	client := initConn(httpsstr)
	defer client.Close()

	proxy := &httpProxy{}
	blockdata, remain, err := proxy.handShake(client)
	hostdata, _ := json.Marshal(hostData{
		Address: "www.sunliver.com",
		Port:    uint16(80),
	})

	assert.Nil(t, err)
	assert.Equal(t, block.ConstBlockTypeConnect, blockdata.Type)
	assert.Equal(t, []byte(hostdata), blockdata.Data)
	assert.Equal(t, "Hello\r\n", string(remain))
}

func TestHTTPSHandShakeFailed(t *testing.T) {
	var err error
	proxy := &httpProxy{}

	// err io.EOF
	msgEOF := "CONNECT www.sunliver.com:80 HTTP/1.1"
	client := initConn(msgEOF)
	defer client.Close()

	_, _, err = proxy.handShake(client)
	assert.Equal(t, io.EOF, err)

	// errBufferFull
	var msgFull string
	for i := 0; i < 1000; i++ {
		msgFull += "CONNECT www.sunliver.com:80 HTTP/1.1"
	}
	clientFull := initConn(msgFull)
	defer client.Close()

	_, _, err = proxy.handShake(clientFull)
	assert.Equal(t, errBufferFull, err)

	// broken handshake msg
	msgBroken := "CONNECT\r\n\r\n"
	clientBroken := initConn(msgBroken)
	defer client.Close()

	_, _, err = proxy.handShake(clientBroken)
	assert.Equal(t, errBrokenMsg, err)
}
