package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/sunliver/shark/lib/block"
)

// https://tools.ietf.org/html/rfc1928
// https://tools.ietf.org/html/rfc1929
// https://ftp.icm.edu.pl/packages/socks/socks4/SOCKS4.protocol

// socks4
// 0x5a: request granted
// 0x5b: request rejected or failed
// 0x5c: request rejected becasue SOCKS server cannot connect to identd on the client
// 0x5d: request rejected because the client program and identd report different user-ids

// socks5
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
	ver byte
	*SocksProxyConf
}

type SocksProxyConf struct {
	Addr        []byte
	Port        []byte
	AuthType    byte
	Credentials map[string]bool
}

const (
	SocksAuthNone     = 0x00
	SocksAuthUserName = 0x02
)

func (p *SocksProxy) HandShake(conn net.Conn) (*block.HostData, error) {
	ver := make([]byte, 1)
	if n, err := io.ReadAtLeast(conn, ver, len(ver)); err != nil || n != len(ver) {
		return nil, fmt.Errorf("err read ver, %v", err)
	}

	p.ver = ver[0]
	switch ver[0] {
	case 0x05:
		return p.socks5HandShake(conn)
	case 0x04:
		return p.socks4HandShake(conn)
	default:
		return nil, fmt.Errorf("unsupported sock version, %v", ver[0])
	}
}

func (p *SocksProxy) socks5HandShake(conn net.Conn) (*block.HostData, error) {
	// socks5 first pkt
	// +----+----------+----------+
	// |VER | NMETHODS | METHODS  |
	// +----+----------+----------+
	// | 1  |    1     | 1 to 255 |
	// +----+----------+----------+

	numMethods := make([]byte, 1)
	if n, err := io.ReadAtLeast(conn, numMethods, len(numMethods)); err != nil || n != len(numMethods) {
		return nil, fmt.Errorf("err read nummethods, %v", err)
	}

	methods := make([]byte, int(numMethods[0]))
	if n, err := io.ReadAtLeast(conn, methods, len(methods)); err != nil || n != len(methods) {
		return nil, fmt.Errorf("err read sock greet methods, %v", err)
	}

	var found bool
	for _, v := range methods {
		if v == p.AuthType {
			found = true
			break
		}
	}

	if !found {
		// no acceptable methods were offered
		_, _ = conn.Write([]byte{0x05, 0xFF})
		return nil, fmt.Errorf("only support No authentication now")
	}

	// send auth negotiation back
	if n, err := conn.Write([]byte{0x05, 0x00}); err != nil || n != 2 {
		return nil, fmt.Errorf("err write greet msg, %v", err)
	}

	if p.AuthType == SocksAuthUserName {
		// +----+------+----------+------+----------+
		// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
		// +----+------+----------+------+----------+
		// | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
		// +----+------+----------+------+----------+

		// first byte is VER
		ulen := make([]byte, 2)
		if n, err := io.ReadAtLeast(conn, ulen, len(ulen)); err != nil || n != len(ulen) {
			return nil, fmt.Errorf("err read ULEN, %v", err)
		}

		if ulen[0] != 0x05 {
			return nil, fmt.Errorf("expected socks ver 5, get %v", ulen[0])
		}

		// least byte is PLEN
		uname := make([]byte, ulen[1]+1)
		if n, err := io.ReadAtLeast(conn, uname, len(uname)); err != nil || n != len(uname) {
			return nil, fmt.Errorf("err read UNAME, %v", err)
		}

		passwd := make([]byte, uname[len(uname)-1])
		if n, err := io.ReadAtLeast(conn, passwd, len(passwd)); err != nil || n != len(passwd) {
			return nil, fmt.Errorf("err read PASSWD, %v", err)
		}

		if _, ok := p.Credentials[string(uname[:len(uname)-1])+":"+string(passwd)]; !ok {
			_, _ = conn.Write([]byte{0x05, 0x01})
			return nil, fmt.Errorf("auth failed")
		}

		_, _ = conn.Write([]byte{0x05, 0x00})
	}

	// +----+-----+-------+------+----------+----------+
	// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	// +----+-----+-------+------+----------+----------+
	// | 1  |  1  | X'00' |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+

	req := make([]byte, 4)
	if n, err := io.ReadAtLeast(conn, req, len(req)); err != nil || n != len(req) {
		return nil, fmt.Errorf("read req header failed, %v", err)
	}

	if req[0] != 0x05 {
		return nil, fmt.Errorf("unsupport sock version, %v", req[0])
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
				return nil, fmt.Errorf("read domain name len failed, %v", err)
			}
			l = int(t[0])
		case 0x04:
			// ipv6
			l = 16
		default:
			return nil, fmt.Errorf("unrecognized ATYP field, %v", req[1])
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
		return nil, fmt.Errorf("unsupprt sock CMD, %v", req[1])
	}

	addr := make([]byte, l)
	if n, err := io.ReadAtLeast(conn, addr, len(addr)); err != nil || n != len(addr) {
		return nil, fmt.Errorf("read DST.ADDR failed, %v", err)
	}
	port := make([]byte, 2)
	if n, err := io.ReadAtLeast(conn, port, len(port)); err != nil || n != len(port) {
		return nil, fmt.Errorf("read DST.PORT failed, %v", err)
	}

	return &block.HostData{
		Address: net.IP(addr).String(),
		Port:    binary.BigEndian.Uint16(port),
	}, nil
}

func (p *SocksProxy) socks4HandShake(conn net.Conn) (*block.HostData, error) {
	// 				+----+----+----+----+----+----+----+----+----+----+....+----+
	// 				| VN | CD | DSTPORT |      DSTIP        | USERID       |NULL|
	// 				+----+----+----+----+----+----+----+----+----+----+....+----+
	// # of bytes:	   1    1      2              4           variable       1

	// just set max length of userID up to 1024-1-2-4-1
	// if not reach EOF, reject the request
	buf := make([]byte, 1024)
	// CD+DSTPORT+DSTIP = 7 bytes
	if n, err := io.ReadAtLeast(conn, buf, 7); err != nil || n == len(buf) {
		// userID too long
		_, _ = conn.Write([]byte{0x00, 0x5d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return nil, err
	}

	buf = buf[:7]

	switch buf[0] {
	case 0x01:
		// CONNECT
		return &block.HostData{
			Address: net.IP(buf[3:7]).String(),
			Port:    binary.BigEndian.Uint16(buf[1:3]),
		}, nil
	case 0x02:
		// BIND
		fallthrough
	default:
		_, _ = conn.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return nil, fmt.Errorf("unsupported cmd, %v", buf[0])
	}
}

func (p *SocksProxy) HandShakeSuccess(conn net.Conn) error {
	switch p.ver {
	case 0x04:
		// 				+----+----+----+----+----+----+----+----+
		// 				| VN | CD | DSTPORT |      DSTIP        |
		// 				+----+----+----+----+----+----+----+----+
		// # of bytes:	   1    1      2              4
		resp := []byte{0x00, 0x5a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if n, err := conn.Write(resp); err != nil || n != len(resp) {
			return err
		}
		return nil
	case 0x05:
		// +----+-----+-------+------+----------+----------+
		// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
		// +----+-----+-------+------+----------+----------+
		// | 1  |  1  | X'00' |  1   | Variable |    2     |
		// +----+-----+-------+------+----------+----------+
		resp := []byte{0x05, 0x00, 0x00, 0x01}
		resp = append(resp, p.Addr...)
		resp = append(resp, p.Port...)
		_, err := conn.Write(resp)
		return err
	}
	return nil
}

func (p *SocksProxy) HandShakeFailed(conn net.Conn) error {
	switch p.ver {
	case 0x04:
		resp := []byte{0x00, 0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		_, err := conn.Write(resp)
		return err
	case 0x05:
		// Connection refused
		resp := []byte{0x05, 0x05, 0x00}
		_, err := conn.Write(resp)
		return err
	}
	return nil
}

func (p *SocksProxy) GetProxyType() ProxyType {
	if p.ver == 0x04 {
		return proxySocks4
	} else {
		return proxySocks5
	}
}
