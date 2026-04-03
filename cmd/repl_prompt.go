package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"
)

func handleP(ctx context.Context, l *readline.Instance) {
	fmt.Println("\n\033[35m=== Entering Chat Mode (Multi-line) ===\033[0m")
	for {
		prompt, ok := readMultiLine(l, "(Ctrl-D to send, Ctrl-C to exit Chat Mode)")
		if !ok {
			break // Exits the Chat Mode loop
		}
		if prompt != "" {
			sendAndRender(ctx, prompt)
		}
	}
	fmt.Println("\033[35m=== Exited Chat Mode ===\033[0m\n")
}

func handlePS(ctx context.Context, l *readline.Instance) {
	clip, err := clipboard.ReadAll()
	if err != nil {
		fmt.Printf("[Error] Cannot read OS clipboard: %v\n", err)
		return
	}
	prompt, ok := readMultiLine(l, "(Enter prompt to prepend to clipboard. Ctrl-D to send, Ctrl-C to cancel)")
	if ok && (prompt != "" || clip != "") {
		finalPrompt := prompt + "\n\n" + clip
		sendAndRender(ctx, finalPrompt)
	}
}

// readMultiLine takes over the readline loop until EOF (Ctrl-D).
func readMultiLine(l *readline.Instance, instructions string) (string, bool) {
	fmt.Printf("\033[90m%s\033[0m\n", instructions)
	var sb strings.Builder
	
	oldPrompt := l.Config.Prompt
	l.SetPrompt("\033[33m>\033[0m ")
	defer l.SetPrompt(oldPrompt)

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt { // Ctrl-C cancels input
			fmt.Println("\033[90m(Cancelled)\033[0m")
			return "", false
		} else if err == io.EOF { // Ctrl-D finishes and sends
			break
		}
		sb.WriteString(line + "\n")
	}
	return strings.TrimSpace(sb.String()), true
}

// sendAndRender abstracts the API call, Context Cancellation, and Glamour rendering
func sendAndRender(ctx context.Context, prompt string) {
	if chatSession == nil {
		fmt.Println("[Error] Cannot send prompt: Gemini API is not initialized.")
		return
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	
	go func() {
		select {
		case <-sigChan:
			fmt.Println("\n\033[31m[!] Request aborted by user.\033[0m")
			cancel() 
		case <-reqCtx.Done():
		}
	}()

	fmt.Println("\033[90mThinking... (Press Ctrl-C to abort)\033[0m")
	resp, err := SendPrompt(reqCtx, prompt)
	
	signal.Stop(sigChan)

	if err != nil {
		if err == context.Canceled {
			return 
		}
		fmt.Printf("[API Error] %v\n", err)
		return
	}

	lastAPIResponse = resp

	out, err := glamour.Render(resp, "dark")
	if err != nil {
		fmt.Printf("\n\033[32mGemini:\033[0m\n%s\n\n", strings.TrimSpace(resp))
	} else {
		fmt.Print(out)
	}
}
