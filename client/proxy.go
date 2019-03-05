package client

import (
	"net"

	"github.com/sunliver/shark/lib/block"
)

const (
	proxyTypeHTTP = iota
	proxyTypeHTTPS
	proxyTypeSocks4
	proxyTypeSocks5
)

type proxy interface {
	// HandShake returns proxy handshake msg
	handShake(conn net.Conn) (blockdata *block.BlockData, read []byte, err error)
	// HandShakeResp returns proxy handshake resp msg
	handShakeResp() []byte
	T() int
}
