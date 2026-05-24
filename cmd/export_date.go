package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exportDateCmd = &cobra.Command{
	Use:   "export-date",
	Short: "Show the global export date for batch processing",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if GlobalSt.ExportDate == "" {
			fmt.Println("Export date is not set.")
		} else {
			fmt.Printf("Current export date: %s\n", GlobalSt.ExportDate)
		}
	},
}

var setExportDateCmd = &cobra.Command{
	Use:   "set-export-date <YYYY-MM-DD>",
	Short: "Set the global export date for batch processing",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		newDate := args[0]
		GlobalSt.ExportDate = newDate
		SaveGlobalState()
		fmt.Printf("[✓] Global export_date set to: %s\n", newDate)
	},
}

func init() {
	rootCmd.AddCommand(exportDateCmd)
	rootCmd.AddCommand(setExportDateCmd)
}
