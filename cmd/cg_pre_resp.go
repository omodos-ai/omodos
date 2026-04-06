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
		if Global.SourceDir == "" || Global.IngestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir and ingest_dir are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "chatgpt")
		fmt.Printf("Starting ChatGPT Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", Global.SourceDir)
		fmt.Printf("Ingest Dir: %s\n", exportDir)

		RunChatGPTExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(cgPrRespCmd)
}
