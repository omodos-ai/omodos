package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"
)

func handleP(ctx context.Context, l *readline.Instance) {
	fmt.Println("\n\033[35m=== Entering Chat Mode (Multi-line) ===\033[0m")
	for {
		prompt, ok := readMultiLine(l, "(Ctrl-D to send, Ctrl-C to exit Chat Mode)")
		if !ok {
			break
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

func readMultiLine(l *readline.Instance, instructions string) (string, bool) {
	fmt.Printf("\033[90m%s\033[0m\n", instructions)
	var sb strings.Builder
	
	oldPrompt := l.Config.Prompt
	l.SetPrompt("\033[33m>\033[0m ")
	defer l.SetPrompt(oldPrompt)

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			fmt.Println("\033[90m(Cancelled)\033[0m")
			return "", false
		} else if err == io.EOF {
			break
		}
		sb.WriteString(line + "\n")
	}
	return strings.TrimSpace(sb.String()), true
}

func sendAndRender(ctx context.Context, prompt string) {
	if chatSession == nil {
		fmt.Println("[Error] Cannot send prompt: Gemini API is not initialized.")
		return
	}

	words := strings.Fields(prompt)
	subjectLength := 5
	if len(words) < 5 { subjectLength = len(words) }
	slug := strings.Join(words[:subjectLength], "_")
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	slug = strings.Trim(reg.ReplaceAllString(slug, "_"), "_")
	if len(slug) > 20 { slug = slug[:20] }
	if slug == "" { slug = "chat" }

	// Unified execution wrapper
	ExecuteTransaction(slug, func(timestamp string, now time.Time) error {
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
		
		Runtime.IsThinking = true
		resp, err := SendPrompt(reqCtx, prompt)
		Runtime.IsThinking = false
		
		signal.Stop(sigChan)

		if err != nil && err != context.Canceled {
			fmt.Printf("[API Error] %v\n", err)
			return err
		} else if err == nil {
			lastAPIResponse = resp
			out, renderErr := glamour.Render(resp, "dark")
			if renderErr != nil {
				fmt.Printf("\n\033[32mGemini:\033[0m\n%s\n\n", strings.TrimSpace(resp))
			} else {
				fmt.Print(out)
			}
			writeMarkdownLedger(now, timestamp, slug, prompt, resp)
		}
		return nil
	})
}

func writeMarkdownLedger(now time.Time, baseTimestamp, slug, prompt, resp string) {
	year := fmt.Sprintf("%04d", now.Year())
	month := fmt.Sprintf("%02d", now.Month())
	filename := fmt.Sprintf("%s-%s.md", baseTimestamp, slug)
	cwd, _ := os.Getwd()

	var md strings.Builder
	md.WriteString("## AI Interaction\n")
	md.WriteString(fmt.Sprintf("### Date\n%s\n", now.Format(time.RFC3339)))
	md.WriteString("### Chat\ndefault\n") 
	md.WriteString("### Source\nggo-cli / Gemini 3.1 Pro\n")
	md.WriteString("### Includes\n-\n") 
	md.WriteString(fmt.Sprintf("### Current working directory\n%s\n", cwd))
	md.WriteString(fmt.Sprintf("### Subject\n%s\n", strings.ReplaceAll(slug, "_", " ")))
	md.WriteString("### Context\n-\n")
	md.WriteString(fmt.Sprintf("### Prompt\n%s\n", prompt))
	md.WriteString(fmt.Sprintf("### Response\n%s\n", resp))

	mdContent := md.String()

	globalDir := filepath.Join(Global.SourceDir, year, month)
	os.MkdirAll(globalDir, 0755)
	os.WriteFile(filepath.Join(globalDir, filename), []byte(mdContent), 0644)

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	projectSourceDir := filepath.Join(centralRepoDir, "chats", "source", year, month)
	os.MkdirAll(projectSourceDir, 0755)
	os.WriteFile(filepath.Join(projectSourceDir, filename), []byte(mdContent), 0644)
}
