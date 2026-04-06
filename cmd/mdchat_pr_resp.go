package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var mdchatPrRespCmd = &cobra.Command{
	Use:   "mdchat-pr-resp",
	Short: "Parses all ggo project chat markdown files into the central source directory",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if Global.SourceDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir is set.")
			os.Exit(1)
		}
		RunMdchatExtract(Global.SourceDir)
	},
}

func RunMdchatExtract(sourceDir string) {
	home, _ := os.UserHomeDir()
	totalInteractions := 0

	fmt.Println("Starting Local Markdown Chat Extraction...")
	fmt.Printf("Source Dir: %s\n", sourceDir)

	for projName := range Global.Projects {
		chatsDir := filepath.Join(home, ".ggo", "ggoai-"+projName, "chats")
		entries, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			chatPath := filepath.Join(chatsDir, entry.Name())
			contentBytes, err := os.ReadFile(chatPath)
			if err != nil {
				continue
			}

			blocks := strings.Split(string(contentBytes), "## AI Interaction")
			for _, block := range blocks {
				if strings.TrimSpace(block) == "" {
					continue
				}

				dIdx := strings.Index(block, "### Date\n")
				cIdx := strings.Index(block, "### Chat\n")
				if dIdx == -1 || cIdx == -1 || cIdx < dIdx {
					continue
				}

				dateStr := strings.TrimSpace(block[dIdx+len("### Date\n") : cIdx])
				parsedTime, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					continue
				}

				// LOCK TO UTC
				parsedTime = parsedTime.UTC()

				sIdx := strings.Index(block, "### Subject\n")
				ctxIdx := strings.Index(block, "### Context\n")
				subjectStr := "interaction"
				if sIdx != -1 && ctxIdx != -1 && ctxIdx > sIdx {
					subjRaw := strings.TrimSpace(block[sIdx+len("### Subject\n") : ctxIdx])
					reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
					slug := reg.ReplaceAllString(subjRaw, "_")
					slug = strings.Trim(slug, "_")
					if len(slug) > 20 {
						slug = strings.TrimRight(slug[:20], "_")
					}
					if slug != "" {
						subjectStr = slug
					}
				}

				year := fmt.Sprintf("%04d", parsedTime.Year())
				month := fmt.Sprintf("%02d", parsedTime.Month())
				timestamp := parsedTime.Format("20060102T150405Z")

				outDir := filepath.Join(sourceDir, year, month)
				os.MkdirAll(outDir, 0755)

				outFilename := fmt.Sprintf("%s-%s.md", timestamp, subjectStr)
				outPath := filepath.Join(outDir, outFilename)

				fullBlock := "## AI Interaction\n" + strings.TrimSpace(block) + "\n\n---\n"
				os.WriteFile(outPath, []byte(fullBlock), 0644)
				totalInteractions++
			}
		}
	}

	fmt.Printf("[✓] Extracted %d local chat interactions directly to %s/\n", totalInteractions, sourceDir)
}

func init() {
	rootCmd.AddCommand(mdchatPrRespCmd)
}
