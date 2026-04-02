package cmd

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

//go:embed scripts/unzip-ingest.sh
var scriptFS embed.FS

var force bool

var unzipCmd = &cobra.Command{
	Use:   "unzip <dir> <date>",
	Short: "Safely unzips Takeout and ChatGPT exports, bypassing corruption",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dir := args[0]
		date := args[1]

		scriptContent, err := scriptFS.ReadFile("scripts/unzip-ingest.sh")
		if err != nil {
			fmt.Printf("Error: Could not read embedded script: %v\n", err)
			os.Exit(1)
		}

		parser := syntax.NewParser()
		prog, err := parser.Parse(strings.NewReader(string(scriptContent)), "unzip-ingest.sh")
		if err != nil {
			fmt.Printf("Error: Failed to parse embedded shell script: %v\n", err)
			os.Exit(1)
		}

		// Let Go handle the flag logic, pass explicit string to bash
		forceStr := "false"
		if force {
			forceStr = "true"
		}
		scriptArgs := []string{dir, date, forceStr}

		runner, err := interp.New(
			interp.StdIO(os.Stdin, os.Stdout, os.Stderr),
			interp.Params(scriptArgs...),
		)
		if err != nil {
			fmt.Printf("Error: Failed to initialize shell interpreter: %v\n", err)
			os.Exit(1)
		}

		err = runner.Run(context.Background(), prog)
		if err != nil {
			fmt.Printf("\nError: Unzip script execution failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(unzipCmd)
	unzipCmd.Flags().BoolVarP(&force, "force", "f", false, "Force unzip and completely wipe existing export directories")
}
