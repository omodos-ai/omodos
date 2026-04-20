package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnowledgeManagerPipeline(t *testing.T) {
	// 1. Establish the hermetic sandbox
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "sources")
	ingestDir := filepath.Join(tempDir, "ingest")
	chunksDir := filepath.Join(tempDir, "chunks")

	Global.SourceDir = sourceDir
	Global.IngestDir = ingestDir
	Global.ChunksDir = chunksDir
	Global.ExportDate = "2026-04-19"

	// 2. Generate Mock Google Takeout
	geminiDir := filepath.Join(ingestDir, "gemini", "2026-04-19-export")
	os.MkdirAll(geminiDir, 0755)
	mockTakeout := `[{"title": "Prompted How do I optimize a RAG pipeline?", "time": "2026-04-19T10:00:00Z", "content": "You should use conceptual projection."}]`
	os.WriteFile(filepath.Join(geminiDir, "MyActivity.json"), []byte(mockTakeout), 0644)

	// 3. Generate Mock ChatGPT (Safely handling Go backticks)
	chatgptDir := filepath.Join(ingestDir, "chatgpt", "2026-04-19-export")
	os.MkdirAll(chatgptDir, 0755)

	ticks := "`" + "`" + "`"
	mockChatGPT := `[{"title": "Debug Session", "create_time": 1713520800, "current_node": "node2", "mapping": {
		"node1": {"id": "node1", "message": {"author": {"role": "user"}, "content": {"parts": ["Fix this bug."]}}},
		"node2": {"id": "node2", "parent": "node1", "message": {"author": {"role": "assistant"}, "content": {"parts": ["Here is the fix:\n` + ticks + `go\nfmt.Println(\"Fixed\")\n` + ticks + `"]}}}
	}}]`
	os.WriteFile(filepath.Join(chatgptDir, "conversations.json"), []byte(mockChatGPT), 0644)

	// 4. Action: Run Extractors
	RunGeminiExtract(sourceDir, geminiDir)
	RunChatGPTExtract(sourceDir, chatgptDir)

	// 5. Verification: Check JSON creation
	var jsonFiles []string
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	// 1 Gemini + 1 ChatGPT record written to both full and cognitive ledgers = 4 records total
	if len(jsonFiles) != 4 {
		t.Fatalf("Expected 4 JSON records (dual written), found %d", len(jsonFiles))
	}

	// 6. Action: Run Chunker (--full)
	fullSourceDir := filepath.Join(sourceDir, "ai-records", "full")
	fullChunksDir := filepath.Join(chunksDir, "full", "2026-04-19")
	RunChunker(fullSourceDir, fullChunksDir, false)

	fullData, _ := os.ReadFile(filepath.Join(fullChunksDir, "chunk-01.md"))
	if !strings.Contains(string(fullData), "Here is the fix:") {
		t.Errorf("Full chunk missing narrative.")
	}

	// 7. Action: Run Chunker (--conceptual)
	cogSourceDir := filepath.Join(sourceDir, "ai-records", "cognitive")
	conceptualChunksDir := filepath.Join(chunksDir, "cognitive", "2026-04-19")
	RunChunker(cogSourceDir, conceptualChunksDir, true)

	conceptualData, _ := os.ReadFile(filepath.Join(conceptualChunksDir, "chunk-01.md"))
	if !strings.Contains(string(conceptualData), "You should use conceptual projection") {
		t.Errorf("Conceptual chunk missing core narrative.")
	}
}
