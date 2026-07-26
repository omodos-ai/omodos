package ggo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/viper"
)

func TestDispatchPromptInterrupt(t *testing.T) {
	// 1. Sandbox Environment Setup
	tempDir, _ := filepath.EvalSymlinks(t.TempDir())
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	core.GlobalCfg = &pb.GlobalConfig{}
	core.GlobalSt = &pb.GlobalState{Projects: make(map[string]string)}
	core.ProjectCfg = &pb.ProjectConfig{ProjectName: "interrupt_test_proj"}
	core.ProjectSt = &pb.ProjectState{}

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()

	os.Setenv("GGO_CONFIG_DIR", filepath.Join(tempDir, ".ggo"))
	defer os.Unsetenv("GGO_CONFIG_DIR")

	projVault := core.GetProjectVaultDir(core.ProjectCfg.ProjectName)
	os.MkdirAll(filepath.Join(projVault, "chats", "interrupt_chat"), 0755)

	// 2. Create a dummy file and enroll it in the context baseline
	testFile := filepath.Join(tempDir, "interrupt_target.txt")
	os.WriteFile(testFile, []byte("INITIAL STATE"), 0644)
	
	core.ProjectSt.IncludedData = []string{testFile}
	core.ActiveChatName = "interrupt_chat"

	// 3. Modify the file to trigger a JIT context diff calculation
	os.WriteFile(testFile, []byte("MODIFIED STATE"), 0644)

	// 4. Mock the API to simulate a network failure or Ctrl-C interrupt
	origSendPrompt := core.SendPrompt
	defer func() { core.SendPrompt = origSendPrompt }()
	
	core.SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		return "", fmt.Errorf("simulated network interrupt")
	}

	// 5. Action: Dispatch the prompt
	promptText := "This is a test prompt that should be saved despite the error."
	_, err := DispatchPrompt(context.Background(), promptText, "test-suite")
	
	if err == nil || !strings.Contains(err.Error(), "simulated network interrupt") {
		t.Fatalf("Expected simulated interrupt error, got: %v", err)
	}

	// 6. Verification 1: Prompt Recorded
	chatDir := filepath.Join(projVault, "chats", "interrupt_chat")
	files, _ := os.ReadDir(chatDir)
	var jsonFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			jsonFiles = append(jsonFiles, f.Name())
		}
	}

	if len(jsonFiles) != 1 {
		t.Fatalf("Expected exactly 1 incomplete JSON record, found %d", len(jsonFiles))
	}

	record, err := core.ReadRecord(filepath.Join(chatDir, jsonFiles[0]))
	if err != nil {
		t.Fatalf("Failed to read saved record: %v", err)
	}

	// Ensure the LLM response is safely nil
	if record.Output != nil && record.Output.Narrative != "" {
		t.Errorf("Expected Output to be nil/empty due to interrupt, but it was populated.")
	}

	// Ensure the user's prompt was saved
	if !strings.Contains(record.Input.Narrative, promptText) {
		t.Errorf("Saved record missing the user prompt.")
	}
	
	// Ensure the JIT context snapshot was captured at the exact moment of interruption
	if !strings.Contains(record.Input.Narrative, "MODIFIED STATE") {
		t.Errorf("Saved record missing the JIT context injection.")
	}

	// 7. Verification 2: Non-progression of Global State
	// The baseline file should NOT exist because the sync was deferred and subsequently aborted
	relPath, _ := filepath.Rel(tempDir, testFile)
	baselinePath := filepath.Join(projVault, "includes", relPath)

	if _, err := os.Stat(baselinePath); !os.IsNotExist(err) {
		t.Errorf("Baseline file was synced to disk despite the API interrupt! The read-modify-write leak is present: %s", baselinePath)
	}
}
