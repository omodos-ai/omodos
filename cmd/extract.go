package cmd

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

//go:embed scripts/extract-refs.sh
var extractScriptFS embed.FS

var extractCmd = &cobra.Command{
	Use:   "extract <source-dir> <infile> <outfile>",
	Short: "Appends raw source interactions to a NotebookLM summary based on timestamps",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		sourceDir := args[0]
		inFile := args[1]
		outFile := args[2]

		fmt.Printf("--- Starting Extraction Pipeline ---\n")
		fmt.Printf("Source Ledger:     %s\n", sourceDir)
		fmt.Printf("Summary File (In): %s\n", inFile)
		fmt.Printf("Full Report (Out): %s\n", outFile)

		// 1. Read embedded script
		scriptContent, err := extractScriptFS.ReadFile("scripts/extract-refs.sh")
		if err != nil {
			fmt.Printf("Error: Could not read embedded script: %v\n", err)
			os.Exit(1)
		}

		parser := syntax.NewParser()
		prog, err := parser.Parse(strings.NewReader(string(scriptContent)), "extract-refs.sh")
		if err != nil {
			fmt.Printf("Error: Failed to parse embedded shell script: %v\n", err)
			os.Exit(1)
		}

		// 2. Prepare the Outfile
		// Open the output file for writing (create or truncate)
		outF, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("Error: Could not open outfile: %v\n", err)
			os.Exit(1)
		}
		defer outF.Close()

		// Read the original NotebookLM summary
		inF, err := os.Open(inFile)
		if err != nil {
			fmt.Printf("Error: Could not open infile: %v\n", err)
			os.Exit(1)
		}

		// Copy the summary into the output file
		_, err = io.Copy(outF, inF)
		inF.Close()
		if err != nil {
			fmt.Printf("Error: Failed to copy summary to outfile: %v\n", err)
			os.Exit(1)
		}

		// Add a clean markdown separator before appending the raw logs
		outF.WriteString("\n\n---\n\n# Raw Source Interactions\n\n")

		// 3. Initialize Daniel Marti's shell interpreter
		// We pass inFile and explicit sourceDir to the script.
		scriptArgs := []string{inFile, sourceDir}
		runner, err := interp.New(
			interp.StdIO(os.Stdin, outF, os.Stderr),
			interp.Params(scriptArgs...),
		)
		if err != nil {
			fmt.Printf("Error: Failed to initialize shell interpreter: %v\n", err)
			os.Exit(1)
		}

		// 4. Execute the script in-memory
		err = runner.Run(context.Background(), prog)
		if err != nil {
			fmt.Printf("\nError: Extract script execution failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("[✓] Extraction complete! Sources appended to output file.")
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
}
