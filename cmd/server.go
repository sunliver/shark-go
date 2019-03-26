package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/profile"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunliver/shark/server"
)

var sPort int
var sAddr string
var sLoglevel int

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().IntVarP(&sPort, "port", "p", 12306, "bind port default=12306")
	serverCmd.Flags().StringVar(&sAddr, "addr", "127.0.0.1", "bind address default=127.0.0.1")
	serverCmd.Flags().IntVar(&sLoglevel, "log-level", 2, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "shark server",
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

		log.SetLevel(log.Level(sLoglevel))

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
