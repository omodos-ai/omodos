package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestChatRecordLifecycle(t *testing.T) {
	// 1. Setup an absolutely isolated filesystem sandbox
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	// 2. Initialize Mock Config & Global State for a "New Project"
	ProjectViper = viper.New()
	GlobalViper = viper.New()

	// Override the sources destination explicitly
	Global.SourceDir = filepath.Join(tempHome, "mock_sources_tree")
	Project.ProjectName = "test_project"
	Global.Projects = map[string]string{
		"test_project": filepath.Join(tempHome, "test_workspace"),
	}

	// 3. Ensure necessary central vault directories exist
	projDir := filepath.Join(tempHome, ".ggo", "ggoai-test_project")
	chatsDir := filepath.Join(projDir, "chats")
	err := os.MkdirAll(chatsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create isolated chats dir: %v", err)
	}

	// Set the active chat context
	ActiveChatName = "test_bug_chat"

	// 4. Mock the Gemini API call so we don't hit the network
	origSendPrompt := SendPrompt
	defer func() { SendPrompt = origSendPrompt }()

	expectedPrompt := "How well is this project structured?"
	expectedResponse := "The architecture is extremely robust and testable."

	SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		return expectedResponse, nil
	}

	// 5. Phase 1: Action (Simulate the REPL processing a prompt)
	// This triggers sendAndRender which should write to BOTH local ledger and global sources
	sendAndRender(context.Background(), expectedPrompt, false, os.Stdout)

	// 6. Phase 1: Verification
	// Walk the mocked Global.SourceDir to verify the file actually landed there
	var foundSourcePath string
	err = filepath.Walk(Global.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			foundSourcePath = path
		}
		return nil
	})

	if err != nil || foundSourcePath == "" {
		t.Fatalf("Bug confirmed: sendAndRender failed to write the record to the <sources> tree at %s", Global.SourceDir)
	}

	// Verify the structural integrity of the initial generated file
	sourceBytes, _ := os.ReadFile(foundSourcePath)
	sourceContent := string(sourceBytes)
	if !strings.Contains(sourceContent, expectedPrompt) || !strings.Contains(sourceContent, expectedResponse) {
		t.Fatalf("Initial source record does not contain the expected prompt/response.\nGot: %s", sourceContent)
	}

	// 7. Phase 2: Action (Test the mdchat-pr-resp extraction parser)
	// Wipe the global sources directory entirely to prove the parser can reconstruct it from the local chat ledger
	os.RemoveAll(Global.SourceDir)
	os.MkdirAll(Global.SourceDir, 0755)

	// Verify the local chat file actually exists before running the parser
	localChatPath := filepath.Join(chatsDir, "test_bug_chat.md")
	if _, err := os.Stat(localChatPath); os.IsNotExist(err) {
		t.Fatalf("Local chat ledger was not created at %s", localChatPath)
	}

	// Execute the command directly
	mdchatPrRespCmd.Run(nil, nil)

	// 8. Phase 2: Verification
	var reconstructedPath string
	err = filepath.Walk(Global.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			reconstructedPath = path
		}
		return nil
	})

	if err != nil || reconstructedPath == "" {
		t.Fatalf("mdchat-pr-resp failed to extract and write the chat record back to the <sources> tree")
	}

	reconBytes, _ := os.ReadFile(reconstructedPath)
	reconContent := string(reconBytes)

	if !strings.Contains(reconContent, expectedPrompt) {
		t.Errorf("Reconstructed source record is missing the prompt:\n%s", reconContent)
	}
	if !strings.Contains(reconContent, expectedResponse) {
		t.Errorf("Reconstructed source record is missing the response:\n%s", reconContent)
	}
	if !strings.Contains(reconContent, "### Date\n") {
		t.Errorf("Reconstructed source record is missing the Date schema block")
	}
}
