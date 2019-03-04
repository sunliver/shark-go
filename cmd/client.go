package cmd

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/client"
)

var clp int
var cprotocol string
var cserver string
var crp int
var cloglevel int
var ccpuprofile bool
var ccoreSz int

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().IntVarP(&clp, "localport", "l", 10087, "local proxy port")
	clientCmd.Flags().StringVarP(&cprotocol, "protocol", "p", "http", "local proxy protocol")
	clientCmd.Flags().StringVarP(&cserver, "server", "s", "localhost", "remote server addr")
	clientCmd.Flags().IntVarP(&crp, "remoteport", "r", 12306, "remote server port")
	clientCmd.Flags().IntVarP(&cloglevel, "loglevel", "g", 3, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
	clientCmd.Flags().BoolVar(&ccpuprofile, "cpuprofile", false, "begin cpu profile")
	clientCmd.Flags().IntVar(&ccoreSz, "coresz", -1, "max num of connections with remote server, -1 is unlimited")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "shark client",
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.Level(cloglevel))

		// if cpuprofile {
		// 	defer profile.Start().Stop()
		// }

		// addr = "localhost"
		// rp = 12306
		// lp = 10087
		// protocol = "http"
		runServer(proxyServerConf{
			RemoteProxyConf: client.RemoteProxyConf{
				// RemotePort:   8654,
				// RemoteServer: "shark.norgerman.com",
				RemotePort:   crp,
				RemoteServer: cserver,
			},
			Port:     clp,
			Protocol: cprotocol,
		})
	},
}

type proxyServerConf struct {
	client.RemoteProxyConf
	Port     int
	Protocol string
}

func runServer(conf proxyServerConf) error {
	m := client.NewManager(conf.RemoteProxyConf, ccoreSz)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Panicf("start server failed, err: %v", err)
		return err
	}
	log.Infof("local port: %v, remote: %v:%v", clp, cserver, crp)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("listener get conn failed, err: %v", err)
			continue
		}

		c := m.GetClient()
		if err != nil {
			log.Errorf("connect to remote server failed, err: %v", err)
			conn.Close()
			continue
		}

		go client.ServerProxy(conf.Protocol, conn, c)
	}

	// TODO gracfully shutdown
	return nil
}
