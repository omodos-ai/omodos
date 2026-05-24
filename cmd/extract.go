package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var overwriteRecords bool

var extractCmd = &cobra.Command{
	Use:   "extract <gemini|chatgpt|mdchat|claude> <ingest-dir>",
	Short: "Extracts conversational AI data directly to the strict JSON ai-records ledger",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		provider := strings.ToLower(args[0])
		ingestDir := args[1]

		if Global.RecordsDir == "" {
			fmt.Println("[Error] Missing global config. Ensure records_dir is set.")
			os.Exit(1)
		}

		switch provider {
		case "gemini":
			fmt.Printf("Starting Strict Gemini Parse Pipeline...\n")
			RunGeminiExtract(Global.RecordsDir, ingestDir, overwriteRecords)
		case "chatgpt":
			fmt.Printf("Starting ChatGPT Parse Pipeline...\n")
			RunChatGPTExtract(Global.RecordsDir, ingestDir, overwriteRecords)
		case "mdchat":
			fmt.Printf("Starting Local Markdown Chat Extraction...\n")
			RunMdchatExtract(Global.RecordsDir, ingestDir, overwriteRecords)
		case "claude":
			fmt.Println("[Info] Claude extraction is planned for a future release.")
		default:
			fmt.Println("[Error] Unknown provider. Use gemini, chatgpt, mdchat, or claude.")
		}
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().BoolVarP(&overwriteRecords, "overwrite", "f", false, "Overwrite existing ai-records in the global ledger")
}
