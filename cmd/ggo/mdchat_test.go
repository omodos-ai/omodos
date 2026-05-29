package ggo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/cmd/raggo"
	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
)

func TestChatRecordLifecycle(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()

	core.GlobalCfg.RecordsDir = filepath.Join(tempHome, "mock_records_tree")
	core.ProjectCfg.ProjectName = "test_project"
	core.GlobalSt.Projects = map[string]string{
		"test_project": filepath.Join(tempHome, "test_workspace"),
	}

	projDir := filepath.Join(tempHome, ".ggo", "ggoai-test_project")
	chatsDir := filepath.Join(projDir, "chats")
	err := os.MkdirAll(chatsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create isolated chats dir: %v", err)
	}

	core.ActiveChatName = "test_bug_chat"

	origSendPrompt := core.SendPrompt
	defer func() { core.SendPrompt = origSendPrompt }()

	expectedPrompt := "How well is this project structured?"
	expectedResponse := "The architecture is extremely robust and testable."

	core.SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		return expectedResponse, nil
	}

	sendAndRender(context.Background(), expectedPrompt, false, os.Stdout)

	var foundRecordPath string
	err = filepath.Walk(core.GlobalCfg.RecordsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			foundRecordPath = path
		}
		return nil
	})

	if err != nil || foundRecordPath == "" {
		t.Fatalf("Bug confirmed: sendAndRender failed to write the JSON record to %s", core.GlobalCfg.RecordsDir)
	}

	recordBytes, _ := os.ReadFile(foundRecordPath)
	recordContent := string(recordBytes)
	if !strings.Contains(recordContent, expectedPrompt) || !strings.Contains(recordContent, expectedResponse) {
		t.Fatalf("Initial JSON record does not contain the expected prompt/response.\nGot: %s", recordContent)
	}

	os.RemoveAll(core.GlobalCfg.RecordsDir)
	os.MkdirAll(core.GlobalCfg.RecordsDir, 0755)

	raggo.RunMdchatExtract(core.GlobalCfg.RecordsDir, chatsDir, false)

	var reconstructedPath string
	err = filepath.Walk(core.GlobalCfg.RecordsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			reconstructedPath = path
		}
		return nil
	})

	if err != nil || reconstructedPath == "" {
		t.Fatalf("mdchat extractor failed to reconstruct the JSON record from the local markdown")
	}

	reconBytes, _ := os.ReadFile(reconstructedPath)
	reconContent := string(reconBytes)

	if !strings.Contains(reconContent, expectedPrompt) {
		t.Errorf("Reconstructed JSON record is missing the prompt:\n%s", reconContent)
	}
	if !strings.Contains(reconContent, expectedResponse) {
		t.Errorf("Reconstructed JSON record is missing the response:\n%s", reconContent)
	}
}
