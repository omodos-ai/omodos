package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestUnifiedPipesAndBroadcasting(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	ProjectViper = viper.New()
	GlobalViper = viper.New()
	ProjectCfg.ProjectName = "pipe_test_project"
	ProjectCfg.CodeDirs = []string{}
	ActiveChatName = "pipe_chat"

	runDir := filepath.Join(tempHome, ".ggo", "ggoai-pipe_test_project", "run")
	chatsDir := filepath.Join(tempHome, ".ggo", "ggoai-pipe_test_project", "chats")
	os.MkdirAll(runDir, 0755)
	os.MkdirAll(chatsDir, 0755)

	// 2. Mock API to prevent network calls
	origSendPrompt := SendPrompt
	defer func() { SendPrompt = origSendPrompt }()
	SendPrompt = func(ctx context.Context, prompt string) (string, error) {
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

	// Start the prompt.pipe listener in a goroutine
	eventChan := make(chan InputEvent, 1)
	go listenPromptPipe(eventChan)

	// Write to prompt.pipe exactly how Emacs would
	pipePath := filepath.Join(runDir, "prompt.pipe")
	err = os.WriteFile(pipePath, []byte("Pipe Prompt"), 0600)
	if err != nil {
		t.Fatalf("Failed to write to prompt pipe: %v", err)
	}

	// Wait for the listener to pick it up and funnel it to the central bus
	var event InputEvent
	select {
	case event = <-eventChan:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for prompt.pipe listener")
	}

	if event.Source != "prompt_pipe" {
		t.Errorf("Expected event source 'prompt_pipe', got '%s'", event.Source)
	}

	// Process the command (Terminal readline = nil, fromPipe = true)
	executeCommand(context.Background(), event.Payload, nil, true)

	// --- Test Scenario B: Input from Terminal ---

	// Process the command directly as if typed in the CLI (Terminal readline = nil, fromPipe = false)
	executeCommand(context.Background(), "Terminal Prompt", nil, false)

	// Allow IO buffers a tiny window to physically flush
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

	// 3. Verify Disk Ledger (chat.md)
	ledgerPath := filepath.Join(chatsDir, "pipe_chat.md")
	ledgerBytes, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("Failed to read ledger file: %v", err)
	}
	ledgerStr := string(ledgerBytes)

	// Check Emacs Event
	if !strings.Contains(ledgerStr, "### Prompt\nPipe Prompt") {
		t.Errorf("Ledger missing Pipe Prompt payload")
	}
	if !strings.Contains(ledgerStr, "### Source\nggo-emacs-pipe") {
		t.Errorf("Ledger failed to correctly tag the pipe source")
	}

	// Check Terminal Event
	if !strings.Contains(ledgerStr, "### Prompt\nTerminal Prompt") {
		t.Errorf("Ledger missing Terminal Prompt payload")
	}
	if !strings.Contains(ledgerStr, "### Source\nggo-cli") {
		t.Errorf("Ledger failed to correctly tag the terminal source")
	}
}
