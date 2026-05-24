package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ggo",
	Short: "Omodos AI - Headless Git-aware Unix orchestrator",
	Long:  `Omodos AI (ggo) manages AI interactions for knowledge management, code generation, and co-authoring with strict data sovereignty.`,

	// This guarantees that our strict Protobuf configuration is fully loaded into
	// memory before ANY subcommand (like 'show costs', 'unzip', or 'chunks') runs.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		InitConfig()
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

func init() {
	// Flags and obsolete initConfig hooks removed.
	// Configuration is now entirely sandboxed and handled by cmd/config.go.
}
