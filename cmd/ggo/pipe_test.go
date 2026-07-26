package ggo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
)

func TestUnifiedPipesAndBroadcasting(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()
	core.ProjectCfg.ProjectName = "pipe_test_project"
	core.ProjectCfg.CodeDirs = []string{}
	core.ActiveChatName = "pipe_chat"

	runDir := filepath.Join(tempHome, ".ggo", "ggoai-pipe_test_project", "run")
	chatsDir := filepath.Join(tempHome, ".ggo", "ggoai-pipe_test_project", "chats", "pipe_chat")
	os.MkdirAll(runDir, 0755)
	os.MkdirAll(chatsDir, 0755)

	// 2. Mock API to prevent network calls
	origSendPrompt := core.SendPrompt
	defer func() { core.SendPrompt = origSendPrompt }()
	core.SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		return "Mock Response for: " + prompt, nil
	}

	// 3. Initialize Pipes via the actual runtime function
	setupPipes()
	defer ResponsePipe.Close()

	// Set up a non-blocking reader for the response pipe to capture the broadcasts
	respReader, err := os.OpenFile(filepath.Join(runDir, "response.pipe"), os.O_RDONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		t.Fatalf("Failed to open response pipe reader: %v", err)
	}
	defer respReader.Close()

	// 4. Capture Stdout (Terminal broadcast)
	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	// --- Test Scenario A: Input from Pipe (Emacs) ---

	eventChan := make(chan InputEvent, 1)
	go listenPromptPipe(eventChan)

	pipePath := filepath.Join(runDir, "prompt.pipe")
	err = os.WriteFile(pipePath, []byte("Pipe Prompt"), 0600)
	if err != nil {
		t.Fatalf("Failed to write to prompt pipe: %v", err)
	}

	var event InputEvent
	select {
	case event = <-eventChan:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for prompt.pipe listener")
	}

	if event.Source != "prompt_pipe" {
		t.Errorf("Expected event source 'prompt_pipe', got '%s'", event.Source)
	}

	executeCommand(context.Background(), event.Payload, nil, true)

	// --- Test Scenario B: Input from Terminal ---

	executeCommand(context.Background(), "prompt Terminal Prompt", nil, false)

	time.Sleep(100 * time.Millisecond)

	// --- Verification Phase ---

	// 1. Verify Terminal Output (Stdout)
	wOut.Close()
	stdoutBytes, _ := io.ReadAll(rOut)
	stdoutStr := string(stdoutBytes)

	if !strings.Contains(stdoutStr, "Mock Response for: Pipe Prompt") {
		t.Errorf("Terminal output missing pipe response.\nGot:\n%s", stdoutStr)
	}
	if !strings.Contains(stdoutStr, "Mock Response for: Terminal Prompt") {
		t.Errorf("Terminal output missing terminal response.\nGot:\n%s", stdoutStr)
	}

	// 2. Verify Response Pipe Output (Emacs Plugin)
	respBytes := make([]byte, 4096)
	n, _ := respReader.Read(respBytes)
	respStr := string(respBytes[:n])

	if !strings.Contains(respStr, "Mock Response for: Pipe Prompt") {
		t.Errorf("Response pipe missing pipe response.\nGot:\n%s", respStr)
	}
	if !strings.Contains(respStr, "Mock Response for: Terminal Prompt") {
		t.Errorf("Response pipe missing terminal response.\nGot:\n%s", respStr)
	}

	// 3. Verify Disk Ledger (.json files)
	files, _ := os.ReadDir(chatsDir)
	var jsonFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			jsonFiles = append(jsonFiles, f.Name())
		}
	}
	sort.Strings(jsonFiles)

	if len(jsonFiles) < 2 {
		t.Fatalf("Expected 2 json records written to disk.")
	}

	r1, _ := core.ReadRecord(filepath.Join(chatsDir, jsonFiles[0]))
	r2, _ := core.ReadRecord(filepath.Join(chatsDir, jsonFiles[1]))

	if !strings.Contains(r1.Input.Narrative, "Pipe Prompt") {
		t.Errorf("Ledger 1 missing Pipe Prompt payload")
	}

	if !strings.Contains(r2.Input.Narrative, "Terminal Prompt") {
		t.Errorf("Ledger 2 missing Terminal Prompt payload")
	}
}
