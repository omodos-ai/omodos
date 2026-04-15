package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const MaxWordsPerChunk = 350000

var chunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "Parses all JSON exports and local chats, chunking the markdown ledger for NotebookLM",
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
			fmt.Printf("[Info] export_date not set, defaulting chunk directory to %s\n", exportDate)
		}

		geminiExport := filepath.Join(ingestDir, "gemini", exportDate+"-export")
		chatgptExport := filepath.Join(ingestDir, "chatgpt", exportDate+"-export")
		
		finalChunksDir := filepath.Join(chunksDir, exportDate)

		fmt.Println("\n=======================================================")
		fmt.Printf(" Running Full Parse & Chunk Pipeline\n")
		fmt.Println("=======================================================")

		fmt.Println("--- Phase 1: Extracting JSON & Local Chats to Markdown ---")
		RunGeminiExtract(sourceDir, geminiExport)
		RunChatGPTExtract(sourceDir, chatgptExport)
		RunMdchatExtract(sourceDir)

		fmt.Printf("\n--- Phase 2: Starting Chunker ---\n")
		fmt.Printf("Source Dir:  %s\n", sourceDir)
		fmt.Printf("Chunks Dir:  %s\n", finalChunksDir)

		RunChunker(sourceDir, finalChunksDir)
	},
}

func init() {
	rootCmd.AddCommand(chunksCmd)
}

func RunChunker(sourceDir, outDir string) {
	var files []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		// FIXED: HasSuffix!
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking source directory:", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("[!] No markdown files found in", sourceDir)
		return
	}

	sort.Strings(files)
	os.MkdirAll(outDir, 0755)

	chunkIndex := 1
	currentWordCount := 0
	var currentChunkBuilder strings.Builder

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(contentBytes)
		words := len(strings.Fields(content))

		if currentWordCount+words > MaxWordsPerChunk && currentWordCount > 0 {
			writeChunk(outDir, chunkIndex, currentChunkBuilder.String())
			chunkIndex++
			currentChunkBuilder.Reset()
			currentWordCount = 0
		}

		currentChunkBuilder.WriteString(content)
		currentChunkBuilder.WriteString("\n\n")
		currentWordCount += words
	}

	if currentWordCount > 0 {
		writeChunk(outDir, chunkIndex, currentChunkBuilder.String())
	}

	fmt.Printf("[✓] Wrote %d chronological chunks to %s\n", chunkIndex, outDir)
}

func writeChunk(outDir string, index int, content string) {
	filename := filepath.Join(outDir, fmt.Sprintf("chunk-%02d.md", index))
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		fmt.Printf("Error writing chunk %s: %v\n", filename, err)
	}
}
