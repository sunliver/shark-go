package client

import (
	"net"
	"testing"
)

func TestHttpProxy_HTTPHandShake(t *testing.T) {
	c, s := net.Pipe()

	go func() {
		_, _ = s.Write([]byte("GET http://en.wikipedia.org:12306/wiki/Proxy_server HTTP/1.1\r\n"))
		_ = s.Close()
	}()

	p := HttpProxy{}
	data, remain, err := p.HandShake(c)
	if err != nil {
		t.Errorf("handshake err, %v", err)
		t.FailNow()
	}

	if p.GetProxyType() != proxyHTTP {
		t.Errorf("handshake parse http schema failed")
		t.FailNow()
	}

	if remain != nil {
		t.Errorf("handshake parse http remain failed")
		t.FailNow()
	}

	if data.Address != "en.wikipedia.org" || data.Port != uint16(12306) {
		t.Errorf("handshake parse hostdata failed, %v", data)
		t.FailNow()
	}
	_ = c.Close()
}

func TestHttpProxy_HTTPSHandShake(t *testing.T) {
	c, s := net.Pipe()

	go func() {
		_, _ = s.Write([]byte("CONNECT example.com:10022 HTTP/1.1\r\n"))
		_ = s.Close()
	}()

	p := HttpProxy{}
	data, remain, err := p.HandShake(c)
	if err != nil {
		t.Errorf("handshake err, %v", err)
		t.FailNow()
	}

	if p.GetProxyType() != proxyHTTPS {
		t.Errorf("handshake parse https schema failed")
		t.FailNow()
	}

	if remain != nil {
		t.Errorf("handshake parse https remain failed")
		t.FailNow()
	}

	if data.Address != "example.com" || data.Port != uint16(10022) {
		t.Errorf("handshake parse hostdata failed, %v", data)
		t.FailNow()
	}
	_ = c.Close()
}
