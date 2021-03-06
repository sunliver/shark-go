package client

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/sunliver/shark/lib/block"
)

var HTTPSuccess = []byte("HTTP/1.1 200 Connection Established\r\n\r\n")

const (
	HTTPMethodConnect = "CONNECT"
)
const (
	constMaxHeaderSzB = 10 * 1024
)

var HTTPCRLF = []byte{'\r', '\n', '\r', '\n'}

type HttpProxy struct {
	https  bool
	remain []byte
}

func (p *HttpProxy) HandShake(conn net.Conn) (data *block.HostData, err error) {
	var read []byte
	var idx int

	// http max url is 10KB
	// try to read 1KB to find the first CR
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("read from conn failed, %v", err)
		}
		read = append(read, buf[:n]...)

		idx = bytes.IndexByte(read, '\n')
		if idx != -1 {
			break
		}

		if len(read) > constMaxHeaderSzB {
			return nil, fmt.Errorf("can not find CR, %v", string(read[:100]))
		}
	}

	// GET http://example.com:12306/wiki/Proxy_server HTTP/1.1
	var method, hostAndPort string
	if _, err := fmt.Sscanf(string(read[:idx]), "%s%s", &method, &hostAndPort); err != nil {
		return nil, fmt.Errorf("parse method, hostAndPort failed, %v, %v", string(read[:100]), err)
	}

	if method == HTTPMethodConnect || strings.HasPrefix(hostAndPort, "https://") {
		p.https = true
		hostAndPort = strings.Replace(hostAndPort, "https://", "", 1)
	} else {
		u, err := url.Parse(hostAndPort)
		if err != nil {
			return nil, fmt.Errorf("parse url failed, %v, %v", string(read[:100]), err)
		}
		hostAndPort = u.Host
		p.remain = read
	}

	// example.com:22
	str := strings.SplitN(hostAndPort, ":", 2)
	port := 80
	addr := str[0]
	if len(str) > 1 {
		port, err = strconv.Atoi(str[1])
		if err != nil {
			return nil, fmt.Errorf("invalid Port, %v, %v", str[1], err)
		}
	}

	return &block.HostData{
		Address: addr,
		Port:    uint16(port),
	}, nil
}

func (p *HttpProxy) HandShakeSuccess(conn net.Conn) error {
	if p.https {
		if n, err := conn.Write(HTTPSuccess); err != nil {
			return err
		} else if n != len(HTTPSuccess) {
			return errors.New("write HTTPSuccess failed")
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (p *HttpProxy) HandShakeFailed(net.Conn) error {
	return nil
}

func (p *HttpProxy) GetProxyType() ProxyType {
	if p.https {
		return proxyHTTPS
	}
	return proxyHTTP
}
