package cmd

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDaemonIPC(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	ProjectViper = viper.New()
	GlobalViper = viper.New()
	Project.ProjectName = "ipc_test_project"
	Project.CodeDirs = []string{}
	ActiveChatName = "ipc_chat"

	runDir := filepath.Join(tempHome, ".ggo", "ggoai-ipc_test_project", "run")

	// Mock the Gemini API so we don't need a real network call for the IPC test
	origSendPrompt := SendPrompt
	defer func() { SendPrompt = origSendPrompt }()
	SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		return "Mock Editor IPC Response", nil
	}

	// 2. Start the Daemon in the background
	InitDaemon()

	// Give the OS 50ms to create the FIFO files physically on disk
	time.Sleep(50 * time.Millisecond)

	// Helper to simulate Emacs writing to a pipe
	writeToPipe := func(pipeName, payload string) {
		pipePath := filepath.Join(runDir, pipeName)
		f, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			t.Fatalf("Failed to open %s for writing: %v", pipeName, err)
		}
		f.WriteString(payload + "\n")
		f.Close()
	}

	// Helper to simulate Emacs reading the response back
	readResponsePipe := func() string {
		pipePath := filepath.Join(runDir, "response.pipe")
		f, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			t.Fatalf("Failed to open response.pipe for reading: %v", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Scan()
		return scanner.Text()
	}

	// --- PHASE 1: Test Prompt Execution ---
	// Start reading BEFORE writing so we don't deadlock the FIFOs
	responseChan := make(chan string)
	go func() {
		responseChan <- readResponsePipe()
	}()

	// Emacs sends a prompt
	writeToPipe("prompt.pipe", "Hello daemon, this is Emacs.")

	// Emacs waits for the LLM response
	response := <-responseChan
	if response != "Mock Editor IPC Response" {
		t.Errorf("Prompt IPC failed. Expected 'Mock Editor IPC Response', got: %s", response)
	}

	// --- PHASE 2: Test Control Signal (WF) ---
	go func() {
		responseChan <- readResponsePipe()
	}()

	// Emacs clicks the "Write Files" shortcut
	writeToPipe("control.pipe", "WF")

	controlResponse := <-responseChan
	// Because the mock response didn't contain XML blocks, WF should gracefully error out
	if !strings.Contains(controlResponse, "[ggo Macro Error] no <file> blocks found") {
		t.Errorf("Control IPC failed. Expected macro error message, got: %s", controlResponse)
	}
}
