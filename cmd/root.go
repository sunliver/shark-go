package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/profile"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	Version string
)

var enableProfile string
var loglevel int
var profileI interface{ Stop() }

func init() {
	rootCmd.PersistentFlags().StringVar(&enableProfile, "profile", "", "cpu, mem, mutex, block, trace")
	rootCmd.PersistentFlags().IntVar(&loglevel, "log-level", 2, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "shark",
	Short:   "shark is a proxy server with self-defined protocol",
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch enableProfile {
		case "cpu":
			profileI = profile.Start(profile.CPUProfile)
		case "mem":
			profileI = profile.Start(profile.MemProfile)
		case "mutex":
			profileI = profile.Start(profile.MutexProfile)
		case "block":
			profileI = profile.Start(profile.BlockProfile)
		case "trace":
			profileI = profile.Start(profile.TraceProfile)
		}

		logrus.SetLevel(logrus.Level(loglevel))
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if profileI != nil {
			profileI.Stop()
		}
	},
}
