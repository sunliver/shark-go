package cmd

import (
	"context"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/server"
)

var sPort int
var sAddr string

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().IntVarP(&sPort, "port", "p", 12306, "bind port")
	serverCmd.Flags().StringVar(&sAddr, "addr", "127.0.0.1", "bind address")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "shark server",
	Run: func(cmd *cobra.Command, args []string) {
		l, err := net.Listen("tcp", fmt.Sprintf("%v:%v", sAddr, sPort))
		if err != nil {
			log.Errorf("listen failed, %v", err)
			return
		}

		log.Infof("now listen %v:%v", sAddr, sPort)

		ctx, _ := context.WithCancel(context.Background())

		for {
			conn, err := l.Accept()
			if err != nil {
				log.Errorf("accept failed, %v", err)
				return
			}

			go server.NewServer(ctx, conn).Run()
		}
	},
}
