package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func TestHermeticEmacsIPC(t *testing.T) {
	// 1. Ensure Emacs is available
	if _, err := exec.LookPath("emacs"); err != nil {
		t.Skip("emacs not found in PATH. Skipping hermetic IPC test.")
	}

	// 2. Setup Hermetic Sandbox & Protect Holy Sources
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)

	origSourceDir := Global.SourceDir
	Global.SourceDir = filepath.Join(tempHome, "mock_sources")

	// Guarantee Restoration (Runs even if the test panics or fails)
	defer func() {
		os.Setenv("HOME", origHome)
		Global.SourceDir = origSourceDir
	}()

	ProjectViper = viper.New()
	GlobalViper = viper.New()
	Project.ProjectName = "emacs_test"
	ActiveChatName = "emacs_chat"

	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-emacs_test", "run"), 0755)
	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-emacs_test", "chats"), 0755)

	// Create config.yaml so the emacs plugin can locate the active project
	configPath := filepath.Join(tempHome, ".ggo", "config.yaml")
	os.WriteFile(configPath, []byte("last_project: emacs_test\n"), 0644)

	// 3. Mock the Gemini API for deterministic test responses
	origSendPrompt := SendPrompt
	defer func() { SendPrompt = origSendPrompt }()

	SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		// Mock a macro-edit payload
		if strings.Contains(prompt, "MACRO_EDIT") {
			targetPath := filepath.Join(tempHome, "emacs_generated.txt")
			return fmt.Sprintf("<file path=\"%s\">\nSUCCESS\n</file>", targetPath), nil
		}
		// Mock standard response
		return "Mock Response: " + prompt, nil
	}

	// 4. Start the ggo daemon (multiplexer event loop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupPipes()
	defer ResponsePipe.Close()

	eventChan := make(chan InputEvent)
	go listenPromptPipe(eventChan)
	go listenControlPipe(eventChan)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				if event.Source == "prompt_pipe" {
					executeCommand(ctx, event.Payload, nil, true)
				} else if event.Source == "control_pipe" {
					if strings.ToUpper(event.Payload) == "WF" {
						handleWF()
					} else {
						executeCommand(ctx, event.Payload, nil, true)
					}
				}
			}
		}
	}()

	// 5. Embed the ERT Lisp script directly into the test file
	ertScript := `
;; Create a dummy markdown-mode if missing to prevent CI/headless crashes
(unless (fboundp 'markdown-mode)
  (define-derived-mode markdown-mode text-mode "Markdown-Mock"))

(require 'ert)
(require 'ggo)

(setq ggo-test-response-buffer "*ggo-chat-test*")

(defun ggo-test-setup ()
  (when (get-buffer ggo-test-response-buffer)
    (with-current-buffer ggo-test-response-buffer
      ;; Explicitly strip the kill-query so Emacs batch doesn't block waiting for "yes/no"
      (when (and (boundp 'ggo-response-process) (processp ggo-response-process))
        (set-process-query-on-exit-flag ggo-response-process nil)
        (delete-process ggo-response-process)))
    (kill-buffer ggo-test-response-buffer))
  
  (with-current-buffer (get-buffer-create ggo-test-response-buffer)
    (ggo-chat-mode)
    (sleep-for 0.5)))

(defun ggo-test-wait (match-string)
  (let ((elapsed 0.0) (found nil))
    (while (and (not found) (< elapsed 5.0))
      (accept-process-output nil 0.2)
      (with-current-buffer ggo-test-response-buffer
        (save-excursion
          (goto-char (point-min))
          (when (search-forward match-string nil t)
            (setq found t))))
      (setq elapsed (+ elapsed 0.2)))
    found))

(ert-deftest ggo-test-01-pipe-communication ()
  "Tests if the prompt pipe correctly triggers a response broadcast."
  (ggo-test-setup)
  (let ((pipe (ggo--get-pipe "prompt.pipe")))
    (write-region "HELLO_GGO\n" nil pipe 'append))
  
  (let ((found (ggo-test-wait "Mock Response: HELLO_GGO")))
    (unless found
      (message "--- Test 01 BUFFER DUMP ---\n%s\n---------------------------" 
               (with-current-buffer ggo-test-response-buffer (buffer-string))))
    (should found)))

(ert-deftest ggo-test-02-macro-wf ()
  "Tests the control pipe's ability to trigger a macro edit file write."
  (ggo-test-setup)
  (let ((pipe (ggo--get-pipe "prompt.pipe")))
    (write-region "MACRO_EDIT\n" nil pipe 'append))
  
  (let ((found (ggo-test-wait "SUCCESS")))
    (unless found
      (message "--- Test 02 BUFFER DUMP ---\n%s\n---------------------------" 
               (with-current-buffer ggo-test-response-buffer (buffer-string))))
    (should found))
  
  ;; Trigger the File Writer (WF) through the control channel
  (ggo-control-send "WF")
  (sleep-for 1.0) ; Wait for Go OS I/O buffer to flush
  
  (should (file-exists-p (expand-file-name "emacs_generated.txt" (getenv "HOME")))))
`
	ertPath := filepath.Join(tempHome, "test.el")
	os.WriteFile(ertPath, []byte(ertScript), 0644)

	// 6. Execute Emacs headlessly using mvdan/sh/v3
	_, thisFile, _, _ := runtime.Caller(0)
	cmdDir := filepath.Dir(thisFile)
	pluginsDir, _ := filepath.Abs(filepath.Join(cmdDir, "..", "plugins"))

	bashScript := fmt.Sprintf(`emacs --batch -L "%s" -l ggo -l "%s" -f ert-run-tests-batch-and-exit`, pluginsDir, ertPath)

	runner, err := interp.New(interp.StdIO(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("Failed to initialize shell interpreter: %v", err)
	}

	prog, err := syntax.NewParser().Parse(strings.NewReader(bashScript), "ert-runner.sh")
	if err != nil {
		t.Fatalf("Failed to parse shell script: %v", err)
	}

	fmt.Println("--- Starting Emacs Batch Execution ---")
	err = runner.Run(context.Background(), prog)
	if err != nil {
		t.Fatalf("Emacs ERT tests failed: %v", err)
	}
}
