package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var annotateCmd = &cobra.Command{
	Use:   "annotate <records-dir> <infile> <outfile>",
	Short: "Appends cleanly formatted source interactions to a NotebookLM summary",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		recordsDir := args[0]
		inFile := args[1]
		outFile := args[2]

		// 1. Read the input markdown file from NotebookLM
		content, err := os.ReadFile(inFile)
		if err != nil {
			fmt.Printf("Error reading infile: %v\n", err)
			return
		}

		// 2. Extract timestamps using Regex
		// Matches formats like 2026-04-19T10:00:00Z or 2026-04-19T100000Z
		re := regexp.MustCompile(`\b20\d{2}-\d{2}-\d{2}T[\d:]+Z?\b`)
		matches := re.FindAllString(string(content), -1)

		if len(matches) == 0 {
			fmt.Println("No timestamps found in the input file.")
			return
		}

		// Deduplicate timestamps
		uniqueTimestamps := make(map[string]bool)
		var timestamps []string
		for _, m := range matches {
			if !uniqueTimestamps[m] {
				uniqueTimestamps[m] = true
				timestamps = append(timestamps, m)
			}
		}

		fmt.Printf("Found %d unique citations. Extracting and projecting records...\n", len(timestamps))

		// 3. Prepare the output file
		out, err := os.Create(outFile)
		if err != nil {
			fmt.Printf("Error creating outfile: %v\n", err)
			return
		}
		defer out.Close()

		// Write the original summary, then append a divider
		out.Write(content)
		out.WriteString("\n\n---\n## Source References\n\n")

		// 4. Find matching JSON records and project to Markdown
		for _, ts := range timestamps {
			// Extract year and month to navigate your YYYY/MM/ folder structure
			if len(ts) < 10 {
				continue
			}
			year := ts[0:4]
			month := ts[5:7]

			searchDir := filepath.Join(recordsDir, year, month)

			files, err := os.ReadDir(searchDir)
			if err != nil {
				continue
			}

			for _, file := range files {
				// Match the timestamp in the JSON filename
				if strings.Contains(file.Name(), strings.ReplaceAll(ts, ":", "")) && strings.HasSuffix(file.Name(), ".json") {
					jsonPath := filepath.Join(searchDir, file.Name())

					jsonData, err := os.ReadFile(jsonPath)
					if err != nil {
						continue
					}

					// Safely parse the JSON without needing the strict Protobuf struct just yet
					var record map[string]interface{}
					if err := json.Unmarshal(jsonData, &record); err != nil {
						fmt.Printf("Warning: Could not parse JSON in %s\n", file.Name())
						continue
					}

					// 5. On-the-fly Projection to Markdown!
					out.WriteString(fmt.Sprintf("### AI Interaction [%s]\n\n", ts))

					// Adjust these keys ("prompt" / "response") if your JSON uses different field names!
					if prompt, ok := record["prompt"].(string); ok {
						out.WriteString(fmt.Sprintf("**Prompt:**\n%s\n\n", prompt))
					} else if userQuery, ok := record["user_query"].(string); ok {
						out.WriteString(fmt.Sprintf("**Prompt:**\n%s\n\n", userQuery))
					}

					if response, ok := record["response"].(string); ok {
						out.WriteString(fmt.Sprintf("**Response:**\n%s\n\n", response))
					} else if aiResponse, ok := record["ai_response"].(string); ok {
						out.WriteString(fmt.Sprintf("**Response:**\n%s\n\n", aiResponse))
					}
					out.WriteString("---\n\n")
				}
			}
		}
		fmt.Printf("[✓] Successfully annotated %s with pure Markdown projections!\n", outFile)
	},
}

func init() {
	rootCmd.AddCommand(annotateCmd)
}
