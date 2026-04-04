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
		if Global.SourceDir == "" || Global.IngestDir == "" || Global.ExportDate == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir, ingest_dir, and export_date are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "gemini", Global.ExportDate+"-export")

		fmt.Printf("Starting Gemini Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", Global.SourceDir)
		fmt.Printf("Export Dir: %s\n", exportDir)

		RunGeminiExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(geminiPrRespCmd)
}
