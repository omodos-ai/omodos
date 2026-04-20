package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiExtractPipeline(t *testing.T) {
	// Setup mock directories
	tempDir := t.TempDir()
	sourceVault := filepath.Join(tempDir, "sources")
	ingestDir := filepath.Join(tempDir, "ingest", "gemini")

	err := os.MkdirAll(ingestDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create ingest dir: %v", err)
	}

	// Mock valid attachment
	attachmentPath := filepath.Join(ingestDir, "system-architecture.pdf")
	os.WriteFile(attachmentPath, []byte("dummy pdf content"), 0644)

	// Mock Gemini MyActivity.json
	mockJSON := `[
		{
			"header": "Gemini Apps",
			"title": "Prompted Write a massive Go function.",
			"time": "2026-04-20T14:30:00Z",
			"safeHtmlItem": [
				{
					"html": "<p>Here is your function:</p><pre><code>func a() {}\nfunc b() {}\nfunc c() {}\nfunc d() {}\nfunc e() {}\nfunc f() {}\nfunc g() {}\nfunc h() {}\nfunc i() {}\nfunc j() {}\nfunc k() {}\nfunc l() {}</code></pre>"
				}
			],
			"attachedFiles": ["system-architecture.pdf"]
		}
	]`

	jsonPath := filepath.Join(ingestDir, "MyActivity.json")
	err = os.WriteFile(jsonPath, []byte(mockJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock JSON: %v", err)
	}

	// Run the extraction
	RunGeminiExtract(sourceVault, ingestDir)

	// Verify paths
	expectedYearMonth := filepath.Join("2026", "04")
	fullDir := filepath.Join(sourceVault, "ai-records", "full", expectedYearMonth)
	cogDir := filepath.Join(sourceVault, "ai-records", "cognitive", expectedYearMonth)

	// Ensure directories were created
	if _, err := os.Stat(fullDir); os.IsNotExist(err) {
		t.Fatalf("Full ledger directory was not created: %s", fullDir)
	}
	if _, err := os.Stat(cogDir); os.IsNotExist(err) {
		t.Fatalf("Cognitive ledger directory was not created: %s", cogDir)
	}

	// Read generated JSON files (we expect 1 file in each)
	files, _ := os.ReadDir(fullDir)
	if len(files) != 1 {
		t.Fatalf("Expected 1 file in full ledger, got %d", len(files))
	}
	filename := files[0].Name()

	fullContent, _ := os.ReadFile(filepath.Join(fullDir, filename))
	cogContent, _ := os.ReadFile(filepath.Join(cogDir, filename))

	// Verify Full Record contains all code
	if !strings.Contains(string(fullContent), "func l() {}") {
		t.Errorf("Full ledger should contain the complete code block")
	}

	// Verify Cognitive Record stripped the > 10 lines code
	if strings.Contains(string(cogContent), "func l() {}") {
		t.Errorf("Cognitive ledger failed to strip large code block")
	}
	if !strings.Contains(string(cogContent), "[Code block > 10 lines stripped for cognitive view]") {
		t.Errorf("Cognitive ledger missing strip placeholder")
	}

	// Verify attachment mapped successfully
	if !strings.Contains(string(fullContent), "system-architecture.pdf") {
		t.Errorf("Failed to map attached file to the AiRecord")
	}
}
