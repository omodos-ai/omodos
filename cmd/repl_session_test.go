package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyMacroEditsUnit(t *testing.T) {
	// 1. Sandbox the environment
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	// Mock config so the ExecuteTransaction hook inside ApplyMacroEdits succeeds
	ProjectViper = viper.New()
	GlobalViper = viper.New()
	Project.ProjectName = "macro_unit_test"
	Project.CodeDirs = []string{} // <-- Added to prevent test leakage!
	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-macro_unit_test"), 0755)

	// 2. Setup mock file paths
	file1Path := filepath.Join(tempHome, "test_file_1.txt")
	file2Path := filepath.Join(tempHome, "nested", "dir", "test_file_2.go")

	// 3. Prepare mock LLM response with conversational text and XML blocks
	mockResponse := fmt.Sprintf(`
Sure, here are the files you requested.

<file path="%s">
Hello World!
This is a test file.
</file>

And the second one in a nested directory:

<file path="%s">
package main
func main() {
	println("hello")
}
</file>

Let me know if you need anything else!
`, file1Path, file2Path)

	// 4. Action
	count, err := ApplyMacroEdits(mockResponse)
	if err != nil {
		t.Fatalf("ApplyMacroEdits returned an unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("ApplyMacroEdits reported %d files written, expected 2", count)
	}

	// 5. Verification
	content1, err := os.ReadFile(file1Path)
	if err != nil {
		t.Fatalf("Failed to read file 1 from disk: %v", err)
	}
	if string(content1) != "Hello World!\nThis is a test file." {
		t.Errorf("File 1 content mismatch.\nGot: %q", string(content1))
	}

	content2, err := os.ReadFile(file2Path)
	if err != nil {
		t.Fatalf("Failed to read file 2 from disk: %v", err)
	}
	if string(content2) != "package main\nfunc main() {\n\tprintln(\"hello\")\n}" {
		t.Errorf("File 2 content mismatch.\nGot: %q", string(content2))
	}
}

func TestLiveApplyMacroEdits(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("Skipping live macro-edit API test: GEMINI_API_KEY environment variable is not set.")
	}

	// 1. Sandbox the environment
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	ProjectViper = viper.New()
	GlobalViper = viper.New()
	Project.ProjectName = "live_macro_test"
	Project.CodeDirs = []string{} // <-- Added to prevent test leakage!
	ActiveChatName = "macro_chat"

	// Create required directories so UpdateChatModel can write GEMINI.md
	projDir := filepath.Join(tempHome, ".ggo", "ggoai-live_macro_test")
	os.MkdirAll(filepath.Join(projDir, "chats"), 0755)

	// 2. Initialize real Gemini API
	ctx := context.Background()
	err := InitGemini(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize Gemini: %v", err)
	}

	// 3. Setup target paths for the LLM to write to
	file1Path := filepath.Join(tempHome, "live_target_1.txt")
	file2Path := filepath.Join(tempHome, "live_target_2.txt")

	// 4. Action: Send a strict prompt testing the XML schema
	prompt := fmt.Sprintf(`I need you to generate two text files using your strict XML <file> schema.
Do not use markdown codeblocks inside the XML.
1. Path: "%s", Content: "Live Test Data 1"
2. Path: "%s", Content: "Live Test Data 2"`, file1Path, file2Path)

	// Send to LLM
	sendAndRender(ctx, prompt)

	// Apply the edits from the in-memory response buffer
	count, err := ApplyMacroEdits(lastAPIResponse)
	if err != nil {
		t.Fatalf("ApplyMacroEdits failed on live response: %v\nResponse was:\n%s", err, lastAPIResponse)
	}
	if count != 2 {
		t.Fatalf("Expected exactly 2 files extracted from live response, got %d.\nResponse was:\n%s", count, lastAPIResponse)
	}

	// 5. Verification
	content1, err := os.ReadFile(file1Path)
	if err != nil {
		t.Fatalf("Failed to read live generated file 1: %v", err)
	}
	if !strings.Contains(string(content1), "Live Test Data 1") {
		t.Errorf("Live generated file 1 did not contain expected text. Got: %q", string(content1))
	}

	content2, err := os.ReadFile(file2Path)
	if err != nil {
		t.Fatalf("Failed to read live generated file 2: %v", err)
	}
	if !strings.Contains(string(content2), "Live Test Data 2") {
		t.Errorf("Live generated file 2 did not contain expected text. Got: %q", string(content2))
	}
}
