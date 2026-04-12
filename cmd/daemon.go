package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "(Deprecated) ggo now uses a unified single-process event loop.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("[Info] The standalone daemon process has been deprecated.")
		fmt.Println("ggo now runs a unified, single-process event loop that automatically handles pipes and terminal input simultaneously.")
		fmt.Println("Please run 'ggo' normally to start the hub.")
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
