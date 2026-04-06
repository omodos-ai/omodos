package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var geminiPrRespCmd = &cobra.Command{
	Use:   "gemini-pr-resp",
	Short: "Parses Gemini JSON exports into the markdown ledger",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if Global.SourceDir == "" || Global.IngestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir and ingest_dir are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "gemini")
		fmt.Printf("Starting Gemini Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", Global.SourceDir)
		fmt.Printf("Ingest Dir: %s\n", exportDir)

		RunGeminiExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(geminiPrRespCmd)
}
