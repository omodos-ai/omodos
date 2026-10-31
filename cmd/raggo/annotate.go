// Copyright 2025-2026 Peter Braam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raggo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
)

var annotateCmd = &cobra.Command{
	Use:   "annotate <infile> <outfile>",
	Short: "Appends cleanly formatted source interactions to a NotebookLM summary",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Architecturally sound: Utilize the globally hydrated config state
		recordsDir := core.GlobalCfg.RecordsDir
		if recordsDir == "" {
			fmt.Println("[Error] Global config 'records_dir' is missing. Set it via 'ggo set records_dir <path>'")
			os.Exit(1)
		}

		inFile := args[0]
		outFile := args[1]

		content, err := os.ReadFile(inFile)
		if err != nil {
			fmt.Printf("[Error] Cannot read input file: %v\n", err)
			os.Exit(1)
		}

		// Safer Regex: Removed \b boundaries to reliably capture [2026-03-24T22:05:01Z] formats
		re := regexp.MustCompile(`20\d{2}-\d{2}-\d{2}T[\d:]+Z?`)
		matches := re.FindAllString(string(content), -1)

		if len(matches) == 0 {
			fmt.Println("[Info] No AI timestamps found in the input file. Outputting unchanged.")
			os.WriteFile(outFile, content, 0644)
			return
		}

		// Deduplicate while preserving sequence stability
		uniqueTimestamps := make(map[string]bool)
		var timestamps []string
		for _, m := range matches {
			if !uniqueTimestamps[m] {
				uniqueTimestamps[m] = true
				timestamps = append(timestamps, m)
			}
		}

		fmt.Printf("Found %d unique citations. Projecting records...\n", len(timestamps))

		out, err := os.Create(outFile)
		if err != nil {
			fmt.Printf("[Error] Cannot create output file: %v\n", err)
			os.Exit(1)
		}
		defer out.Close()

		out.Write(content)
		out.WriteString("\n\n---\n## Source References\n\n")

		for _, ts := range timestamps {
			if len(ts) < 10 {
				continue
			}
			year := ts[0:4]
			month := ts[5:7]
			searchDir := filepath.Join(recordsDir, year, month)

			files, err := os.ReadDir(searchDir)
			if err != nil {
				fmt.Printf("[Warning] Ledger directory missing: %s\n", searchDir)
				continue
			}

			// Sanitize the requested timestamp to match the safe-slug filename format
			cleanTS := strings.ReplaceAll(ts, ":", "")
			cleanTS = strings.ReplaceAll(cleanTS, "-", "")

			for _, file := range files {
				if strings.Contains(file.Name(), cleanTS) && strings.HasSuffix(file.Name(), ".json") {
					jsonPath := filepath.Join(searchDir, file.Name())

					// Use shared binary core to safely unmarshal directly to pb.AiRecord
					record, err := core.ReadRecord(jsonPath)
					if err != nil {
						fmt.Printf("[Warning] Schema mismatch or corrupt record %s: %v\n", file.Name(), err)
						continue
					}

					out.WriteString(fmt.Sprintf("### AI Interaction [%s]\n\n", ts))

					if record.Input != nil && record.Input.Narrative != "" {
						out.WriteString(fmt.Sprintf("**Prompt:**\n%s\n\n", record.Input.Narrative))
					}

					if record.Output != nil && record.Output.Narrative != "" {
						out.WriteString(fmt.Sprintf("**Response:**\n%s\n\n", record.Output.Narrative))
					}
					out.WriteString("---\n\n")
				}
			}
		}
		fmt.Printf("[✓] Annotation complete. Wrote to %s\n", outFile)
	},
}

func init() {
	rootCmd.AddCommand(annotateCmd)
}
