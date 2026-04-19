package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/cobra"
)

const MaxWordsPerChunk = 350000

var conceptualMode bool

var chunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "Projects JSON AiRecords into NotebookLM-ready Markdown chunks",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		sourceDir := Global.SourceDir
		chunksDir := Global.ChunksDir
		ingestDir := Global.IngestDir

		if sourceDir == "" || chunksDir == "" || ingestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir, chunks_dir, and ingest_dir are set.")
			os.Exit(1)
		}

		exportDate := Global.ExportDate
		if exportDate == "" {
			exportDate = time.Now().Format("2006-01-02")
		}

		geminiExport := filepath.Join(ingestDir, "gemini", exportDate+"-export")
		chatgptExport := filepath.Join(ingestDir, "chatgpt", exportDate+"-export")

		subDir := "full"
		if conceptualMode {
			subDir = "conceptual"
		}
		finalChunksDir := filepath.Join(chunksDir, subDir, exportDate)

		fmt.Println("\n=======================================================")
		fmt.Printf(" Running Full Parse & Chunk Pipeline (%s mode)\n", subDir)
		fmt.Println("=======================================================")

		fmt.Println("--- Phase 1: Extracting Legacy Exports to Structured JSON ---")
		RunGeminiExtract(sourceDir, geminiExport)
		RunChatGPTExtract(sourceDir, chatgptExport)

		fmt.Printf("\n--- Phase 2: Projecting Records to Markdown ---\n")
		fmt.Printf("Source Vault: %s\n", sourceDir)
		fmt.Printf("Chunks Dir:   %s\n", finalChunksDir)

		RunChunker(sourceDir, finalChunksDir)
	},
}

func init() {
	rootCmd.AddCommand(chunksCmd)
	chunksCmd.Flags().BoolVarP(&conceptualMode, "conceptual", "c", false, "Omit snippets, code, and diffs. Extract pure intellectual narrative only.")
}

func RunChunker(sourceDir, outDir string) {
	var jsonFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking source directory:", err)
		return
	}

	if len(jsonFiles) == 0 {
		fmt.Println("[!] No JSON records found in", sourceDir)
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
			continue
		}

		md := ProjectToMarkdown(record, conceptualMode)
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

	// Nil safety checks since Protobuf sub-messages are pointers
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
		sb.WriteString(r.Input.Narrative + "\n\n")

		if len(r.Input.Attachments) > 0 {
			sb.WriteString("*(Attached Multimodal Documents: ")
			var attNames []string
			for _, a := range r.Input.Attachments {
				attNames = append(attNames, a.FileName)
			}
			sb.WriteString(strings.Join(attNames, ", ") + ")*\n\n")
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
		sb.WriteString(r.Output.Narrative + "\n\n")

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

func writeChunk(outDir string, index int, content string) {
	filename := filepath.Join(outDir, fmt.Sprintf("chunk-%02d.md", index))
	os.WriteFile(filename, []byte(content), 0644)
}
