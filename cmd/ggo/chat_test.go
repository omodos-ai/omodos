package ggo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/viper"
)

func TestLiveEndToEndChatAndContext(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		userHome, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(userHome, "secrets", "gemini-token")); os.IsNotExist(err) {
			t.Skip("Skipping live API test: No env var and no secrets file found.")
		}
	}

	tempDir, _ := filepath.EvalSymlinks(t.TempDir())

	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	core.GlobalCfg = &pb.GlobalConfig{}
	core.GlobalSt = &pb.GlobalState{Projects: make(map[string]string)}
	core.ProjectCfg = &pb.ProjectConfig{}
	core.ProjectSt = &pb.ProjectState{}

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()

	origConfigDir := os.Getenv("GGO_CONFIG_DIR")
	os.Setenv("GGO_CONFIG_DIR", filepath.Join(tempDir, ".ggo"))
	defer func() {
		if origConfigDir == "" {
			os.Unsetenv("GGO_CONFIG_DIR")
		} else {
			os.Setenv("GGO_CONFIG_DIR", origConfigDir)
		}
	}()

	core.ProjectCfg.ProjectName = "live_chat_test"
	os.MkdirAll(filepath.Join(tempDir, ".ggo", "ggoai-live_chat_test", "chats"), 0755)

	testFile := filepath.Join(tempDir, "system_config.txt")

	// --- Stage 1: Baseline Creation ---
	os.WriteFile(testFile, []byte("STATUS=OFFLINE\nVERSION=1.0"), 0644)

	lastAPIResponse = "" // strict reset
	script1 := "inc " + testFile + "\ncc live_test_chat\nprompt What is the current STATUS in system_config.txt? Answer with just the value.\nexit\n"
	runScript(script1)

	chatPath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "chats", "live_test_chat.md")

	contentBytes, err := os.ReadFile(chatPath)
	if err != nil {
		t.Fatalf("Failed to find generated chat file: %v", err)
	}

	blocks := strings.Split(string(contentBytes), "## AI Interaction")
	if len(blocks) < 2 {
		t.Fatalf("Expected at least one AI interaction block. (If this failed, the 5-retry backoff was defeated by Google)")
	}
	stage1Block := blocks[len(blocks)-1]

	if !strings.Contains(strings.ToUpper(stage1Block), "OFFLINE") {
		t.Fatalf("Gemini failed to read the baseline context. Expected 'OFFLINE' in response. Got:\n%s", stage1Block)
	}

	// --- Stage 2: The JIT Full-State Modification ---
	os.WriteFile(testFile, []byte("STATUS=ONLINE\nVERSION=2.0"), 0644)

	lastAPIResponse = "" // strict reset
	script2 := "cc live_test_chat\nprompt I just updated the config on disk. What is the STATUS now? Answer with just the value.\nexit\n"
	runScript(script2)

	contentBytes, _ = os.ReadFile(chatPath)
	blocks = strings.Split(string(contentBytes), "## AI Interaction")
	if len(blocks) < 3 {
		t.Fatalf("Expected at least two AI interaction blocks. (If this failed, the 5-retry backoff was defeated by Google)")
	}
	stage2Block := blocks[len(blocks)-1]

	if !strings.Contains(stage2Block, "<system_context>") {
		t.Fatalf("Expected XML <system_context> injection, but it was missing.")
	}

	rIdx := strings.Index(stage2Block, "### Response\n")
	if rIdx == -1 {
		t.Fatalf("No response found in stage 2 block")
	}
	responseStr := stage2Block[rIdx:]

	if !strings.Contains(strings.ToUpper(responseStr), "ONLINE") {
		t.Fatalf("Gemini failed to comprehend the JIT XML update. Expected 'ONLINE'. Response block:\n%s", responseStr)
	}
}
