package raggo

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

//go:embed scripts/unzip-ingest.sh
var scriptFS embed.FS

var force bool

var unzipCmd = &cobra.Command{
	Use:   "unzip <gemini|chatgpt> <zip-file-path> <YYYY-MM-DD>",
	Short: "Explicitly unzips a Takeout or ChatGPT export and sets the global export date",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		provider := strings.ToLower(args[0])
		zipPath := args[1]
		date := args[2]

		if provider != "gemini" && provider != "chatgpt" {
			fmt.Println("[Error] Provider must be 'gemini' or 'chatgpt'.")
			os.Exit(1)
		}

		if core.GlobalCfg.IngestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure 'ingest_dir' is set in config.yaml.")
			os.Exit(1)
		}

		// 1. Safely set export date in STATE (not CONFIG!)
		core.GlobalSt.ExportDate = date
		core.SaveGlobalState()
		fmt.Printf("[✓] Global export_date set to: %s (Saved to state.json)\n", date)

		// 2. Pre-check for existing directory to prevent accidental wipes
		targetDir := filepath.Join(core.GlobalCfg.IngestDir, provider, date+"-export")
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) && !force {
			fmt.Printf("[Error] Target directory already exists: %s\n", targetDir)
			fmt.Println("        Use the -f flag to wipe and force overwrite.")
			os.Exit(1)
		}

		// 3. Execute the shell script
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

		forceStr := "false"
		if force {
			forceStr = "true"
		}

		scriptArgs := []string{provider, zipPath, core.GlobalCfg.IngestDir, date, forceStr}

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
	unzipCmd.Flags().BoolVarP(&force, "force", "f", false, "Force unzip and completely wipe existing export directory")
}
