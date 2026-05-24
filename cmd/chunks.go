package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/cobra"
)

const MaxWordsPerChunk = 350000

var chunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "Projects JSON AiRecords into NotebookLM-ready Markdown chunks (Full & Cognitive)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		recordsDir := Global.RecordsDir
		chunksDir := Global.ChunksDir

		if recordsDir == "" || chunksDir == "" {
			fmt.Println("[Error] Missing global config. Ensure records_dir and chunks_dir are set.")
			os.Exit(1)
		}

		exportDate := Global.ExportDate
		if exportDate == "" {
			exportDate = time.Now().Format("2006-01-02")
		}

		fullChunks := filepath.Join(chunksDir, "full", exportDate)
		cogChunks := filepath.Join(chunksDir, "cognitive", exportDate)

		fmt.Println("\n=======================================================")
		fmt.Printf(" Running Dual Chunk Pipeline (%s)\n", exportDate)
		fmt.Println("=======================================================")

		fmt.Println("\n--- Phase 1: Projecting Full Records ---")
		RunChunker(recordsDir, fullChunks, false)

		fmt.Println("\n--- Phase 2: Projecting Cognitive Records ---")
		RunChunker(recordsDir, cogChunks, true)
	},
}

func init() {
	rootCmd.AddCommand(chunksCmd)
}

func RunChunker(recordsDir, outDir string, conceptual bool) {
	var jsonFiles []string
	err := filepath.Walk(recordsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking records directory:", err)
		return
	}

	if len(jsonFiles) == 0 {
		fmt.Printf("[!] No JSON records found in %s\n", recordsDir)
		return
	}

	sort.Strings(jsonFiles)
	os.MkdirAll(outDir, 0755)

	chunkIndex := 1
	currentWordCount := 0
	var currentChunkBuilder strings.Builder

	for _, file := range jsonFiles {
		record, err := ReadRecord(file)
		if err != nil {
			continue // Gracefully skip if it's a version from the future
		}

		md := ProjectToMarkdown(record, conceptual)
		words := len(strings.Fields(md))

		if currentWordCount+words > MaxWordsPerChunk && currentWordCount > 0 {
			writeChunk(outDir, chunkIndex, currentChunkBuilder.String())
			chunkIndex++
			currentChunkBuilder.Reset()
			currentWordCount = 0
		}

		currentChunkBuilder.WriteString(md)
		currentChunkBuilder.WriteString("\n\n---\n\n")
		currentWordCount += words
	}

	if currentWordCount > 0 {
		writeChunk(outDir, chunkIndex, currentChunkBuilder.String())
	}

	fmt.Printf("[✓] Wrote %d chronological chunks to %s\n", chunkIndex, outDir)
}

func ProjectToMarkdown(r *pb.AiRecord, conceptual bool) string {
	var sb strings.Builder

	timestamp := "Unknown"
	chatId := "Unknown"
	modelName := "Unknown"
	if r.Metadata != nil {
		timestamp = r.Metadata.TimestampUtc
		chatId = r.Metadata.ChatId
		modelName = r.Metadata.ModelName
	}

	sb.WriteString(fmt.Sprintf("## AI Interaction [%s]\n", timestamp))
	sb.WriteString(fmt.Sprintf("**Chat:** %s | **Model:** %s\n\n", chatId, modelName))

	sb.WriteString("### Prompt\n")
	if r.Input != nil {
		narrative := r.Input.Narrative
		if conceptual {
			narrative = stripLargeCodeBlocks(narrative)
		}
		sb.WriteString(narrative + "\n\n")

		if len(r.Input.Attachments) > 0 {
			var attNames []string
			for _, a := range r.Input.Attachments {
				attNames = append(attNames, a.FileName)
			}

			if len(attNames) <= 3 {
				sb.WriteString(fmt.Sprintf("*(Attached Multimodal Documents: %s)*\n\n", strings.Join(attNames, ", ")))
			} else {
				preview := strings.Join(attNames[:3], ", ")
				hiddenCount := len(attNames) - 3
				sb.WriteString(fmt.Sprintf("*(Attached Multimodal Documents: %s ... and %d more files)*\n\n", preview, hiddenCount))
			}
		}

		if !conceptual {
			if len(r.Input.Snippets) > 0 {
				sb.WriteString("### User Snippets\n")
				for _, s := range r.Input.Snippets {
					sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", s.Language, s.Content))
				}
			}
			if len(r.Input.JitContext) > 0 {
				sb.WriteString("### Modified Desk Context\n")
				for _, diff := range r.Input.JitContext {
					sb.WriteString(fmt.Sprintf("**File:** `%s`\n```\n%s\n```\n\n", diff.Path, diff.Content))
				}
			}
		}
	}

	sb.WriteString("### Response\n")
	if r.Output != nil {
		narrative := r.Output.Narrative
		if conceptual {
			narrative = stripLargeCodeBlocks(narrative)
		}
		sb.WriteString(narrative + "\n\n")

		if !conceptual {
			if len(r.Output.Snippets) > 0 {
				sb.WriteString("### Generated Snippets\n")
				for _, s := range r.Output.Snippets {
					sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", s.Language, s.Content))
				}
			}
			if len(r.Output.MacroEdits) > 0 {
				sb.WriteString("### Generated File Modifications\n")
				for _, diff := range r.Output.MacroEdits {
					sb.WriteString(fmt.Sprintf("**File:** `%s`\n```\n%s\n```\n\n", diff.Path, diff.Content))
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

func stripLargeCodeBlocks(text string) string {
	re := regexp.MustCompile("(?s)```(.*?)```")
	return re.ReplaceAllStringFunc(text, func(match string) string {
		lines := strings.Split(match, "\n")
		if len(lines) > 12 {
			return "```\n[Code block > 10 lines stripped for cognitive view]\n```"
		}
		return match
	})
}

func writeChunk(outDir string, index int, content string) {
	filename := filepath.Join(outDir, fmt.Sprintf("chunk-%02d.md", index))
	os.WriteFile(filename, []byte(content), 0644)
}
