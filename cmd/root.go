package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "shark",
	Short:   "shark is a proxy server with self-defined protocol",
	Version: "1.0.0-rc1",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var enableProfile string

func init() {
	rootCmd.PersistentFlags().StringVar(&enableProfile, "profile", "", "cpu, mem, mutex, block, trace")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
