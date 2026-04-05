package cmd

import (
	"bufio"
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

	pipes := []string{"prompt.pipe", "control.pipe", "shell-1.pipe", "shell-2.pipe", "shell-3.pipe"}

	for _, p := range pipes {
		pipePath := filepath.Join(runDir, p)
		// Clean up stale pipes from previous crashes
		os.Remove(pipePath)

		// Create the Named Pipe (FIFO)
		err := syscall.Mkfifo(pipePath, 0600)
		if err != nil {
			fmt.Printf("[Warning] Failed to create pipe %s: %v\n", p, err)
			continue
		}

		// Spin up a concurrent listener for each pipe
		go listenPipe(pipePath, p)
	}
}

func listenPipe(pipePath string, pipeName string) {
	for {
		// os.OpenFile blocks until a writer connects to the pipe
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
		// TODO: Wire to Gemini API and active_chat.md
		fmt.Printf("\n\033[90m[daemon:prompt] Received %d bytes. (API routing pending)...\033[0m\nggo> ", len(payload))
	default:
		if strings.HasPrefix(pipeName, "shell-") {
			// TODO: Store shell blob in memory buffer for the next prompt
			fmt.Printf("\n\033[90m[daemon:%s] Captured %d bytes of shell output.\033[0m\nggo> ", pipeName, len(payload))
		}
	}
}

func handleControlSignal(signal string) {
	signal = strings.ToUpper(strings.TrimSpace(signal))
	switch signal {
	case "INTERRUPT":
		fmt.Print("\n\033[33m[daemon] Intercepted INTERRUPT signal. Halting AI stream...\033[0m\nggo> ")
		// TODO: Trigger context cancellation for Gemini API
	default:
		fmt.Printf("\n\033[90m[daemon] Unknown control signal: %s\033[0m\nggo> ", signal)
	}
}

func CleanupDaemon() {
	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run")
	// Sweep the pipes so they don't litter the filesystem when ggo exits
	files, _ := filepath.Glob(filepath.Join(runDir, "*.pipe"))
	for _, f := range files {
		os.Remove(f)
	}
}
