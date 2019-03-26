package client

import (
	"net"

	"github.com/sunliver/shark/lib/block"
)

type ProxyType int

const (
	proxyHTTP   ProxyType = iota
	proxyHTTPS  ProxyType = iota
	proxySocks4 ProxyType = iota
	proxySocks5 ProxyType = iota
)

type Proxy interface {
	// HandShake returns Proxy handshake msg
	HandShake(net.Conn) (*block.HostData, []byte, error)
	// HandShakeResp returns Proxy handshake resp msg
	HandShakeSuccess(net.Conn) error
	HandShakeFailed(net.Conn) error
	GetProxyType() ProxyType
}
