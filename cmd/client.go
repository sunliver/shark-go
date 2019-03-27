package cmd

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/client"
)

var claddr string
var clport int
var cprotocol string
var craddr string
var crport int
var ccoreSz int
var cauth string

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().StringVar(&claddr, "local-addr", "127.0.0.1", "local addr to listen")
	clientCmd.Flags().IntVar(&clport, "local-port", 10087, "local proxy port")
	clientCmd.Flags().StringVar(&cprotocol, "protocol", "http", "local proxy protocol, http or socks(v4 and v5)")
	clientCmd.Flags().StringVar(&craddr, "remote-addr", "127.0.0.1", "remote server addr")
	clientCmd.Flags().IntVar(&crport, "remote-port", 12306, "remote server port")
	clientCmd.Flags().IntVar(&ccoreSz, "coresz", 4, "max num of connections with remote server")
	clientCmd.Flags().StringVar(&cauth, "auth", "", "socks5 basic auth, RFC 1929. Format with username:passwd, separated by ;")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "shark client",
	Run: func(cmd *cobra.Command, args []string) {
		var sockProxyConf client.SocksProxyConf
		if cprotocol == "socks" {
			ip := net.ParseIP(claddr)
			if ipv4 := ip.To4(); ipv4 != nil {
				sockProxyConf.Addr = ipv4
			} else if ipv6 := ip.To16(); ipv6 != nil {
				sockProxyConf.Addr = ipv6
			} else {
				sockProxyConf.Addr = []byte(claddr)
			}

			buf := make([]byte, 2)
			binary.BigEndian.PutUint16(buf, uint16(clport))
			sockProxyConf.Port = buf

			if cauth != "" {
				sockProxyConf.AuthType = client.SocksAuthUserName
				sockProxyConf.Credentials = make(map[string]bool)

				list := strings.Split(cauth, ";")
				for _, v := range list {
					sockProxyConf.Credentials[v] = true
				}
			}
		}

		l, err := net.Listen("tcp", fmt.Sprintf("%v:%v", claddr, clport))
		if err != nil {
			log.Panicf("start client failed, %v", err)
		}

		log.Infof("listen %v:%v, remote: %v:%v", claddr, clport, craddr, crport)
		m := client.NewManager(ccoreSz, fmt.Sprintf("%v:%v", craddr, crport))
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Errorf("l get conn failed, err: %v", err)
				continue
			}

			go m.Start(conn, NewProxy(&sockProxyConf))
		}
	},
}

func NewProxy(conf *client.SocksProxyConf) client.Proxy {
	if cprotocol == "socks" {
		return &client.SocksProxy{SocksProxyConf: conf}
	} else {
		return &client.HttpProxy{}
	}
}
