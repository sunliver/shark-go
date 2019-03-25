package client

import (
	"net"

	"github.com/sunliver/shark/lib/block"
)

type proxyType int

const (
	proxyHTTP   proxyType = iota
	proxyHTTPS  proxyType = iota
	proxySocks4 proxyType = iota
	proxySocks5 proxyType = iota
)

type proxy interface {
	// HandShake returns proxy handshake msg
	HandShake(conn net.Conn) (*block.HostData, []byte, error)
	// HandShakeResp returns proxy handshake resp msg
	HandShakeResp() []byte
	GetProxyType() proxyType
}
