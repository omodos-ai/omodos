package ggo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func TestHermeticEmacsIPC(t *testing.T) {
	if _, err := exec.LookPath("emacs"); err != nil {
		t.Skip("emacs not found in PATH. Skipping hermetic IPC test.")
	}

	tempHome := t.TempDir()

	t.Cleanup(func() {
		filepath.Walk(tempHome, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				os.Chmod(path, 0777)
			}
			return nil
		})
	})

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)

	origRecordsDir := core.GlobalCfg.RecordsDir
	core.GlobalCfg.RecordsDir = filepath.Join(tempHome, "mock_records")

	defer func() {
		os.Setenv("HOME", origHome)
		core.GlobalCfg.RecordsDir = origRecordsDir
	}()

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()
	core.ProjectCfg.ProjectName = "emacs_test"
	core.ActiveChatName = "emacs_chat"

	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-emacs_test", "run"), 0755)
	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-emacs_test", "chats"), 0755)

	configPath := filepath.Join(tempHome, ".ggo", "config.yaml")
	os.WriteFile(configPath, []byte("last_project: emacs_test\n"), 0644)

	origSendPrompt := core.SendPrompt
	defer func() { core.SendPrompt = origSendPrompt }()

	core.SendPrompt = func(ctx context.Context, prompt string) (string, error) {
		if strings.Contains(prompt, "MACRO_EDIT") {
			targetPath := filepath.Join(tempHome, "emacs_generated.txt")
			return fmt.Sprintf("<file path=\"%s\">\nSUCCESS\n</file>", targetPath), nil
		}
		return "Mock Response: " + prompt, nil
	}

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
					payloadParts := strings.Fields(event.Payload)
					payloadCmd := strings.ToUpper(payloadParts[0])
					if payloadCmd == "WF" || payloadCmd == "FW" {
						handleFileRouter("fw", payloadParts)
					} else {
						executeCommand(ctx, event.Payload, nil, true)
					}
				}
			}
		}
	}()

	ertScript := `
(unless (fboundp 'markdown-mode)
  (define-derived-mode markdown-mode text-mode "Markdown-Mock"))

(require 'ert)
(require 'ggo)

(setq ggo-test-response-buffer "*ggo-chat-test*")

(defun ggo-test-setup ()
  (when (get-buffer ggo-test-response-buffer)
    (with-current-buffer ggo-test-response-buffer
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
  (ggo-test-setup)
  (let ((pipe (ggo--get-pipe "prompt.pipe")))
    (write-region "HELLO_GGO\n" nil pipe 'append))
  (should (ggo-test-wait "Mock Response: HELLO_GGO")))

(ert-deftest ggo-test-02-macro-wf ()
  (ggo-test-setup)
  (let ((pipe (ggo--get-pipe "prompt.pipe")))
    (write-region "MACRO_EDIT\n" nil pipe 'append))
  (should (ggo-test-wait "SUCCESS"))
  
  (ggo-control-send "WF")
  (sleep-for 1.0)
  
  (should (file-exists-p (expand-file-name "emacs_generated.txt" (getenv "HOME")))))
`
	ertPath := filepath.Join(tempHome, "test.el")
	os.WriteFile(ertPath, []byte(ertScript), 0644)

	_, thisFile, _, _ := runtime.Caller(0)
	cmdDir := filepath.Dir(thisFile)
	pluginsDir, _ := filepath.Abs(filepath.Join(cmdDir, "..", "..", "plugins"))

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
