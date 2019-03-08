package cmd

import (
	"fmt"
	"net"

	"github.com/pkg/profile"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/client"
)

var clp int
var cprotocol string
var cserver string
var crp int
var cloglevel int
var ccoreSz int

func init() {
	rootCmd.AddCommand(clientCmd)

	clientCmd.Flags().IntVarP(&clp, "localport", "l", 10087, "local proxy port")
	clientCmd.Flags().StringVarP(&cprotocol, "protocol", "p", "http", "local proxy protocol, currently only support http")
	clientCmd.Flags().StringVarP(&cserver, "server", "s", "127.0.0.1", "remote server addr")
	clientCmd.Flags().IntVarP(&crp, "remoteport", "r", 12306, "remote server port")
	clientCmd.Flags().IntVarP(&cloglevel, "loglevel", "g", 2, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
	clientCmd.Flags().IntVar(&ccoreSz, "coresz", 4, "max num of connections with remote server")
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

		listener, err := net.Listen("tcp", fmt.Sprintf(":%v", clp))
		if err != nil {
			panic(fmt.Errorf("start client failed, %v", err))
		}
		log.Infof("local port: %v, remote: %v:%v", clp, cserver, crp)

		m := client.NewManager(ccoreSz, fmt.Sprintf("%v:%v", cserver, crp))
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Errorf("listener get conn failed, err: %v", err)
				continue
			}

			go m.Run(conn, "http")
		}
	},
}
