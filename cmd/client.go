package cmd

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"github.com/pkg/profile"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/client"
)

var claddr string
var clport int
var cprotocol string
var craddr string
var crport int
var cloglevel int
var ccoreSz int
var cauth string

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().StringVar(&claddr, "local-addr", "127.0.0.1", "local addr to listen. (currently only support ipv4 addr)")
	clientCmd.Flags().IntVar(&clport, "local-port", 10087, "local proxy port")
	clientCmd.Flags().StringVar(&cprotocol, "protocol", "http", "local proxy protocol, `http` or `socks`(4 and 5 is supported)")
	clientCmd.Flags().StringVar(&craddr, "remote-addr", "127.0.0.1", "remote server addr")
	clientCmd.Flags().IntVar(&crport, "remote-port", 12306, "remote server port")
	clientCmd.Flags().IntVar(&cloglevel, "log-level", 2, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
	clientCmd.Flags().IntVar(&ccoreSz, "coresz", 4, "max num of connections with remote server")
	clientCmd.Flags().StringVar(&cauth, "auth", "", "socks5 basic auth, RFC 1929. Format with `username:passwd`, separated by `;`")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "shark client",
	Run: func(cmd *cobra.Command, args []string) {
		switch enableProfile {
		case "cpu":
			defer profile.Start(profile.CPUProfile).Stop()
		case "mem":
			defer profile.Start(profile.MemProfile).Stop()
		case "mutex":
			defer profile.Start(profile.MutexProfile).Stop()
		case "block":
			defer profile.Start(profile.BlockProfile).Stop()
		case "trace":
			defer profile.Start(profile.TraceProfile).Stop()
		}

		log.SetLevel(log.Level(cloglevel))

		var sockProxyConf client.SocksProxyConf
		if cprotocol == "socks" {
			sockProxyConf.Addr = net.ParseIP(claddr).To4()
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

		listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", claddr, clport))
		if err != nil {
			panic(fmt.Errorf("start client failed, %v", err))
		}
		log.Infof("listen %v:%v, remote: %v:%v", claddr, clport, craddr, crport)

		m := client.NewManager(ccoreSz, fmt.Sprintf("%v:%v", craddr, crport))
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Errorf("listener get conn failed, err: %v", err)
				continue
			}

			go m.Run(conn, NewProxy(&sockProxyConf))
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
