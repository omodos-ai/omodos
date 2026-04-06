package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func InitDaemon() {
	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run")
	os.MkdirAll(runDir, 0755)

	// Added response.pipe for bidirectional IPC
	pipes := []string{"prompt.pipe", "control.pipe", "response.pipe", "shell-1.pipe", "shell-2.pipe"}

	for _, p := range pipes {
		pipePath := filepath.Join(runDir, p)
		os.Remove(pipePath) // Sweep stale pipes

		err := syscall.Mkfifo(pipePath, 0600)
		if err != nil {
			fmt.Printf("[Warning] Failed to create pipe %s: %v\n", p, err)
			continue
		}

		// Only spawn reader goroutines for incoming pipes!
		// The daemon exclusively writes to the response.pipe.
		if p != "response.pipe" {
			go listenPipe(pipePath, p)
		}
	}
}

func listenPipe(pipePath string, pipeName string) {
	for {
		// Blocks until Editor (or Test) opens the pipe to write
		file, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		var payloadBuilder strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			payloadBuilder.WriteString(line + "\n")
		}
		file.Close()

		payload := strings.TrimSpace(payloadBuilder.String())
		if payload == "" {
			continue
		}

		// Route the payload based on the pipe it came from
		routePayload(pipeName, payload)
	}
}

func routePayload(pipeName, payload string) {
	switch pipeName {
	case "control.pipe":
		handleControlSignal(payload)
	case "prompt.pipe":
		// Offload API call to a goroutine so the daemon doesn't block other pipes
		go func(p string) {
			resp, err := DispatchPrompt(context.Background(), p, "ggo-editor-ipc")
			if err != nil {
				sendToResponsePipe(fmt.Sprintf("[ggo API Error]\n%v\n", err))
			} else {
				sendToResponsePipe(resp + "\n")
			}
		}(payload)
	default:
		if strings.HasPrefix(pipeName, "shell-") {
			fmt.Printf("\n\033[90m[daemon:%s] Captured %d bytes of shell output.\033[0m\nggo> ", pipeName, len(payload))
		}
	}
}

func handleControlSignal(signal string) {
	signal = strings.ToUpper(strings.TrimSpace(signal))
	switch signal {
	case "ABORT":
		// TODO: Trigger context cancellation
		sendToResponsePipe("[ggo Control] ABORT received. AI stream halted.\n")
	case "END_CHAT":
		sendToResponsePipe("[ggo Control] Shutting down session.\n")
		CleanupDaemon()
		os.Exit(0)
	case "WRITE_FILES", "WF":
		if lastAPIResponse != "" {
			count, err := ApplyMacroEdits(lastAPIResponse)
			if err != nil {
				sendToResponsePipe(fmt.Sprintf("[ggo Macro Error] %v\n", err))
			} else {
				sendToResponsePipe(fmt.Sprintf("[ggo Control] Successfully extracted and applied %d file modification(s).\n", count))
			}
		} else {
			sendToResponsePipe("[ggo Warning] Ignored WF signal: No recent AI response in memory.\n")
		}
	default:
		sendToResponsePipe(fmt.Sprintf("[ggo Warning] Unknown control signal: %s\n", signal))
	}
}

// sendToResponsePipe pushes data to Emacs. If Emacs isn't listening,
// it drops the payload rather than hanging the entire ggo daemon.
func sendToResponsePipe(msg string) {
	home, _ := os.UserHomeDir()
	pipePath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run", "response.pipe")

	// Open O_WRONLY blocks until Emacs opens it O_RDONLY.
	file, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("\n\033[33m[daemon] Editor not reading response.pipe. Payload dropped.\033[0m\nggo> ")
		return
	}
	defer file.Close()
	file.WriteString(msg)
}

func CleanupDaemon() {
	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run")
	files, _ := filepath.Glob(filepath.Join(runDir, "*.pipe"))
	for _, f := range files {
		os.Remove(f)
	}
}
