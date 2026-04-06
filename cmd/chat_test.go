package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveEndToEndChatAndContext(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("Skipping live API test: GEMINI_API_KEY environment variable is not set.")
	}

	// 1. Initialize real configuration based on the current working directory
	InitConfig()

	if Project.ProjectName == "" {
		t.Skip("Skipping live test: not running from within a configured ggo project root.")
	}

	// Create a temporary source_dir for the test so we don't pollute the actual global directory
	tempSourceDir, _ := os.MkdirTemp(".", "tmp-source-dir-*")
	origSourceDir := Global.SourceDir
	Global.SourceDir = tempSourceDir

	// 2. Setup local sandbox inside the real working directory
	tempDir, err := os.MkdirTemp(".", "tmp-chat-test-*")
	if err != nil {
		t.Fatalf("Failed to create local temp dir: %v", err)
	}

	// Clean up filesystem and config pollution when the test finishes
	origIncludes := append([]string(nil), Project.IncludedData...)
	defer func() {
		os.RemoveAll(tempDir)
		os.RemoveAll(tempSourceDir)
		Global.SourceDir = origSourceDir

		Project.IncludedData = origIncludes
		home, _ := os.UserHomeDir()
		ProjectViper.Set("included_data", Project.IncludedData)
		ProjectViper.WriteConfigAs(filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "project.yaml"))

		os.Remove(filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats", "live_test_chat.md"))
	}()

	testFile := filepath.Join(tempDir, "system_config.txt")

	// --- Stage 1: Baseline Creation ---
	os.WriteFile(testFile, []byte("STATUS=OFFLINE\nVERSION=1.0"), 0644)

	// Include the file, start chat, and ask Gemini to read the baseline state
	script1 := "inc " + testFile + "\ncc live_test_chat\nWhat is the current STATUS in system_config.txt? Answer with just the value.\nexit\n"
	runScript(script1)

	home, _ := os.UserHomeDir()
	chatPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats", "live_test_chat.md")

	contentBytes, err := os.ReadFile(chatPath)
	if err != nil {
		t.Fatalf("Failed to find generated chat file: %v", err)
	}

	blocks := strings.Split(string(contentBytes), "## AI Interaction")
	if len(blocks) < 2 {
		t.Fatalf("Expected at least one AI interaction block.")
	}
	stage1Block := blocks[len(blocks)-1]

	if !strings.Contains(strings.ToUpper(stage1Block), "OFFLINE") {
		t.Fatalf("Gemini failed to read the baseline context. Expected 'OFFLINE' in response. Got:\n%s", stage1Block)
	}

	// --- Stage 2: The JIT Full-State Modification ---
	// Modify the file on disk without explicitly telling the LLM what changed
	os.WriteFile(testFile, []byte("STATUS=ONLINE\nVERSION=2.0"), 0644)

	script2 := "cc live_test_chat\nI just updated the config on disk. What is the STATUS now? Answer with just the value.\nexit\n"
	runScript(script2)

	contentBytes, _ = os.ReadFile(chatPath)
	blocks = strings.Split(string(contentBytes), "## AI Interaction")
	stage2Block := blocks[len(blocks)-1]

	// Verify the XML payload was injected correctly into the schema locally
	if !strings.Contains(stage2Block, "<system_context>") {
		t.Fatalf("Expected XML <system_context> injection, but it was missing.")
	}
	if !strings.Contains(stage2Block, "STATUS=ONLINE") {
		t.Fatalf("Expected the modified full text to be in the JIT payload, but it wasn't.")
	}

	// Verify State Update block was appended
	if !strings.Contains(stage2Block, "### State update") {
		t.Fatalf("Expected '### State update' to be appended to the schema.")
	}

	// VERIFY COGNITIVE UNDERSTANDING: Check Gemini's actual response
	rIdx := strings.Index(stage2Block, "### Response\n")
	if rIdx == -1 {
		t.Fatalf("No response found in stage 2 block")
	}
	responseStr := stage2Block[rIdx:]

	if !strings.Contains(strings.ToUpper(responseStr), "ONLINE") || strings.Contains(strings.ToUpper(responseStr), "OFFLINE") {
		t.Fatalf("Gemini failed to comprehend the JIT XML context update. Expected 'ONLINE'. Response block:\n%s", responseStr)
	}
}
