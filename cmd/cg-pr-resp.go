package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cgPrRespCmd = &cobra.Command{
	Use:   "cg-pr-resp",
	Short: "Parses ChatGPT JSON exports into the markdown ledger",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if Global.SourceDir == "" || Global.IngestDir == "" || Global.ExportDate == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir, ingest_dir, and export_date are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "chatgpt", Global.ExportDate+"-export")

		fmt.Printf("Starting ChatGPT Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", Global.SourceDir)
		fmt.Printf("Export Dir: %s\n", exportDir)

		RunChatGPTExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(cgPrRespCmd)
}
