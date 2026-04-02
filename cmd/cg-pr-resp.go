package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var cgPrRespCmd = &cobra.Command{
	Use:   "cg-pr-resp <source_dir> <export_dir>",
	Short: "Parses ChatGPT JSON exports into the markdown ledger",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceDir := args[0]
		exportDir := args[1]

		fmt.Printf("Starting ChatGPT Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", sourceDir)
		fmt.Printf("Export Dir: %s\n", exportDir)

		RunChatGPTExtract(sourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(cgPrRespCmd)
}
