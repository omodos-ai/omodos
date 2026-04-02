package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var geminiPrRespCmd = &cobra.Command{
	Use:   "gemini-pr-resp <source_dir> <export_dir>",
	Short: "Parses Gemini JSON exports into the markdown ledger",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceDir := args[0]
		exportDir := args[1]

		fmt.Printf("Starting Gemini Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", sourceDir)
		fmt.Printf("Export Dir: %s\n", exportDir)

		RunGeminiExtract(sourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(geminiPrRespCmd)
}
