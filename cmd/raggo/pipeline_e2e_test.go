package raggo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
)

func TestKnowledgeManagerPipeline(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	os.Setenv("GGO_CONFIG_DIR", filepath.Join(tempDir, ".ggo"))
	defer os.Unsetenv("GGO_CONFIG_DIR")

	os.MkdirAll(filepath.Join(tempDir, ".ggo"), 0755)

	stateContent := fmt.Sprintf(`{"projects":{"raggo_test_project":"%s"}}`, tempDir)
	os.WriteFile(filepath.Join(tempDir, ".ggo", "state.json"), []byte(stateContent), 0644)

	recordsDir := filepath.Join(tempDir, "records")
	ingestDir := filepath.Join(tempDir, "ingest")
	chunksDir := filepath.Join(tempDir, "chunks")

	core.InitConfig("raggo_test_project")
	core.GlobalCfg.RecordsDir = recordsDir
	core.GlobalCfg.IngestDir = ingestDir
	core.GlobalCfg.ChunksDir = chunksDir
	core.GlobalSt.ExportDate = "2026-04-19"

	geminiDir := filepath.Join(ingestDir, "gemini", "2026-04-19-export")
	os.MkdirAll(geminiDir, 0755)
	mockTakeout := `[{"title": "Prompted How do I optimize a RAG pipeline?", "time": "2026-04-19T10:00:00Z", "content": "You should use conceptual projection."}]`
	os.WriteFile(filepath.Join(geminiDir, "MyActivity.json"), []byte(mockTakeout), 0644)

	chatgptDir := filepath.Join(ingestDir, "chatgpt", "2026-04-19-export")
	os.MkdirAll(chatgptDir, 0755)

	// Cleanly inject markdown backticks into the JSON payload
	ticks := "`" + "`" + "`"
	mockChatGPT := fmt.Sprintf(`[{"title": "Debug Session", "create_time": 1713520800, "current_node": "node2", "mapping": {
		"node1": {"id": "node1", "message": {"author": {"role": "user"}, "content": {"parts": ["Fix this bug."]}}},
		"node2": {"id": "node2", "parent": "node1", "message": {"author": {"role": "assistant"}, "content": {"parts": ["Here is the fix:\n%sgo\nfmt.Println(\"Fixed\")\n%s"]}}}
	}}]`, ticks, ticks)

	os.WriteFile(filepath.Join(chatgptDir, "conversations.json"), []byte(mockChatGPT), 0644)

	RunGeminiExtract(recordsDir, geminiDir, false)
	RunChatGPTExtract(recordsDir, chatgptDir, false)

	var jsonFiles []string
	filepath.Walk(recordsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})

	if len(jsonFiles) != 2 {
		t.Fatalf("Expected exactly 2 JSON records (No dual writes), found %d", len(jsonFiles))
	}

	fullChunksDir := filepath.Join(chunksDir, "full", "2026-04-19")
	RunChunker(recordsDir, fullChunksDir, false)

	fullData, _ := os.ReadFile(filepath.Join(fullChunksDir, "chunk-01.md"))
	if !strings.Contains(string(fullData), "Here is the fix:") {
		t.Errorf("Full chunk missing narrative.")
	}

	conceptualChunksDir := filepath.Join(chunksDir, "cognitive", "2026-04-19")
	RunChunker(recordsDir, conceptualChunksDir, true)

	conceptualData, _ := os.ReadFile(filepath.Join(conceptualChunksDir, "chunk-01.md"))
	if !strings.Contains(string(conceptualData), "You should use conceptual projection") {
		t.Errorf("Conceptual chunk missing core narrative.")
	}
}
