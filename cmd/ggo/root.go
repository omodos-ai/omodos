package ggo

import (
	"fmt"
	"os"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ggo",
	Short: "Omodos AI Coding Assistant",
	Long:  "ggo manages Git-aware AI coding interactions, Time Machine transactions, and context management.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.InitConfig()
	},
	Run: func(cmd *cobra.Command, args []string) {
		RunREPL()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
