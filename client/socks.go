package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/sunliver/shark/lib/block"
)

// X'00' succeeded
// X'01' general SOCKS server failure
// X'02' connection not allowed by ruleset
// X'03' Network unreachable
// X'04' Host unreachable
// X'05' Connection refused
// X'06' TTL expired
// X'07' Command not supported
// X'08' Address type not supported
// X'09' to X'FF' unassigned

type SocksProxy struct {
	atyp byte
	Addr []byte
	Port []byte
}

func (p *SocksProxy) HandShake(conn net.Conn) (*block.HostData, []byte, error) {
	// +----+----------+----------+
	// |VER | NMETHODS | METHODS  |
	// +----+----------+----------+
	// | 1  |    1     | 1 to 255 |
	// +----+----------+----------+
	buf := make([]byte, 2)
	if n, err := io.ReadAtLeast(conn, buf, len(buf)); err != nil || n != len(buf) {
		return nil, nil, fmt.Errorf("err read sock greet, %v", err)
	}

	if buf[0] != 0x05 {
		return nil, nil, fmt.Errorf("unsupported sock version, %v", buf[0])
	}

	methods := make([]byte, int(buf[1]))
	if n, err := io.ReadAtLeast(conn, buf, len(methods)); err != nil || n != len(methods) {
		return nil, nil, fmt.Errorf("err read sock greet methods, %v", err)
	}

	var found bool
	for _, v := range methods {
		if v == 0x00 {
			found = true
			break
		}
	}

	if !found {
		// no acceptable methods were offered
		_, _ = conn.Write([]byte{0x05, 0xFF})
		return nil, nil, fmt.Errorf("only support No authentication now")
	}

	// send greet back
	if n, err := conn.Write([]byte{0x05, 0x00}); err != nil || n != 2 {
		return nil, nil, fmt.Errorf("err write greet msg, %v", err)
	}

	// +----+-----+-------+------+----------+----------+
	// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+

	req := make([]byte, 4)
	if n, err := io.ReadAtLeast(conn, req, len(req)); err != nil || n != len(req) {
		return nil, nil, fmt.Errorf("read req header failed, %v", err)
	}

	if req[0] != 0x05 {
		return nil, nil, fmt.Errorf("unsupport sock version, %v", req[0])
	}

	var l int
	switch req[1] {
	case 0x01:
		// CONNECT
		switch req[3] {
		case 0x01:
			// ipv4
			l = 4
		case 0x03:
			// domain name
			t := make([]byte, 1)
			if n, err := io.ReadAtLeast(conn, t, len(t)); err != nil || n != len(t) {
				return nil, nil, fmt.Errorf("read domain name len failed, %v", err)
			}
			l = int(t[0])
		case 0x04:
			// ipv6
			l = 16
		default:
			return nil, nil, fmt.Errorf("unrecognized ATYP field, %v", req[1])
		}
	case 0x02:
		// BIND
		fallthrough
	case 0x03:
		// UDP ASSOCIATE
		fallthrough
	default:
		// Command not supported
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00})
		return nil, nil, fmt.Errorf("unsupprt sock CMD, %v", req[1])
	}
	p.atyp = req[1]

	addr := make([]byte, l)
	if n, err := io.ReadAtLeast(conn, addr, len(addr)); err != nil || n != len(addr) {
		return nil, nil, fmt.Errorf("read DST.ADDR failed, %v", err)
	}
	port := make([]byte, 2)
	if n, err := io.ReadAtLeast(conn, port, len(port)); err != nil || n != len(port) {
		return nil, nil, fmt.Errorf("read DST.PORT failed, %v", err)
	}

	return &block.HostData{
		Address: net.IP(addr).String(),
		Port:    binary.BigEndian.Uint16(port),
	}, nil, nil
}

func (p *SocksProxy) HandShakeSuccess(conn net.Conn) error {
	// +----+-----+-------+------+----------+----------+
	// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+
	resp := []byte{0x05, 0x00, 0x00, p.atyp}
	resp = append(resp, p.Addr...)
	resp = append(resp, p.Port...)
	if n, err := conn.Write(resp); err != nil || n != len(resp) {
		return err
	}
	return nil
}

func (p *SocksProxy) HandShakeFailed(conn net.Conn) error {
	// Connection refused
	resp := []byte{0x05, 0x05, 0x00}
	_, err := conn.Write(resp)
	return err
}

func (p *SocksProxy) GetProxyType() ProxyType {
	return proxySocks5
}
