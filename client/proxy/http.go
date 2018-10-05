package proxy

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/sunliver/shark-go/protocol"
)

const (
	constHTTPMethodConnect = "CONNECT"
	constHTTPSuccess       = "HTTP/1.1 200 Connection Established\r\n\r\n"
)

var errHTTPDegrade = errors.New("proxy: degrade to HTTP")
var errBufferFull = errors.New("proxy: buffer full")
var errBrokenMsg = errors.New("proxy: broken msg")

const (
	constMaxHeaderSzB = 10 * 1024
)

type httpProxy struct {
	pType int
}

func (p *httpProxy) handShake(conn net.Conn) (blockdata *protocol.BlockData, remain []byte, err error) {
	var msg []byte
	var read []byte
Found:
	for {
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil {
			log.Errorf("[HTTPProxy] invalid handshake msg, read: %v, err: %v", string(read), err)
			return nil, nil, err
		}

		read = append(read, buf[:n]...)

		// stop when reach \r\n\r\n
		l := len(buf)
		for k, v := range buf {
			if v == 0x0a && l > k+2 && buf[k+1] == 0x0d && buf[k+2] == 0x0a {
				msg = read[:len(read)-n+k+2+1]
				break Found
			}
		}

		if len(read) > constMaxHeaderSzB {
			log.Errorf("[HTTPProxy] read %v header can not find stop words", constMaxHeaderSzB)
			return nil, nil, errBufferFull
		}
	}

	strs := strings.Split(string(msg), " ")
	if len(strs) < 2 {
		log.Errorf("[HTTPProxy] Invalid handshake msg, %v", msg)
		return nil, nil, errBrokenMsg
	}

	if strs[0] != constHTTPMethodConnect {
		strs[1] = strings.TrimLeft(strs[1], "http://")
		err = errHTTPDegrade
		remain = read
		p.pType = constProxyTypeHTTP
	} else {
		if len(read) > len(msg) {
			remain = read[len(msg):]
		}
		p.pType = constProxyTypeHTTPS
	}

	hosts := strings.SplitN(strs[1], ":", 2)
	address := strings.SplitN(hosts[0], "/", 2)[0]

	port := 80
	if len(hosts) == 2 {
		port, _ = strconv.Atoi(hosts[1])
	}

	hostdata, _ := json.Marshal(hostData{
		Address: address,
		Port:    uint16(port),
	})

	return &protocol.BlockData{
		Type: protocol.ConstBlockTypeConnect,
		Data: []byte(hostdata),
	}, remain, err
}

func (p *httpProxy) handShakeResp() []byte {
	return []byte(constHTTPSuccess)
}

func (p *httpProxy) proxyType() int {
	return p.pType
}
