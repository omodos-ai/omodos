package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NotebookLM limit is ~500,000 words per document. 
// We use 350,000 to leave a massive safety buffer for metadata and attachments.
const MaxWordsPerChunk = 350000 

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run chunker.go <source_dir> <out_dir>")
		os.Exit(1)
	}

	sourceDir := os.Args[1]
	outDir := os.Args[2]

	// 1. Collect all markdown files
	var files []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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

	// 2. Sort files alphabetically.
	// Since our files are named YYYYMMDDThhmmss-subject.md, this naturally 
	// interleaves Gemini and ChatGPT into one perfect chronological timeline!
	sort.Strings(files)

	// 3. Prepare output directory
	os.MkdirAll(outDir, 0755)

	// 4. Stream and Chunk
	chunkIndex := 1
	currentWordCount := 0
	var currentChunkBuilder strings.Builder

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		content := string(contentBytes)
		words := len(strings.Fields(content)) // Fast approximation of word count

		// If this file pushes us over the limit, write the current chunk to disk and reset
		if currentWordCount+words > MaxWordsPerChunk && currentWordCount > 0 {
			writeChunk(outDir, chunkIndex, currentChunkBuilder.String())
			chunkIndex++
			currentChunkBuilder.Reset()
			currentWordCount = 0
		}

		currentChunkBuilder.WriteString(content)
		currentChunkBuilder.WriteString("\n\n") // Add breathing room between prompts
		currentWordCount += words
	}

	// Write whatever is left in the buffer to the final chunk
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
