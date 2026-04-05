package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/chzyer/readline"
)

var ActiveChatName string

// GetActiveChat ensures we always know what chat we are in, even across reboots
func GetActiveChat() string {
	if ActiveChatName != "" {
		return ActiveChatName
	}
	if ProjectViper.IsSet("active_chat") {
		ActiveChatName = ProjectViper.GetString("active_chat")
		return ActiveChatName
	}
	return "default"
}

func handleCC(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: cc <chat-name>")
		return
	}
	ActiveChatName = parts[1]

	// 1. Persist the active chat to the project config
	ProjectViper.Set("active_chat", ActiveChatName)
	home, _ := os.UserHomeDir()
	ProjectViper.WriteConfigAs(filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "project.yaml"))

	chatDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats")
	os.MkdirAll(chatDir, 0755)

	filePath := filepath.Join(chatDir, "chat-"+ActiveChatName+".md")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		header := fmt.Sprintf("# 🧠 ggo Chat: %s\n*Started: %s*\n\n", ActiveChatName, time.Now().Format("2006-01-02 15:04:05"))
		os.WriteFile(filePath, []byte(header), 0644)
		fmt.Printf("[✓] Created and activated chat: %s\n", ActiveChatName)
	} else {
		fmt.Printf("[Info] Activated existing chat: %s\n", ActiveChatName)
	}

	// 2. Hydration Hook: Re-initialize the API to load the new chat's history
	UpdateChatModel()
}

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
	fmt.Println("\033[35m=== Exited Chat Mode ===\033[0m")
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

func buildJITContextAndSync() string {
	if len(Project.IncludedData) == 0 {
		return "-"
	}

	var xmlBuilder strings.Builder
	modifiedCount := 0
	home, _ := os.UserHomeDir()
	centralIncludesDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "includes")
	cwd, _ := os.Getwd()

	for _, targetPath := range Project.IncludedData {
		filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}

			absPath, _ := filepath.Abs(path)
			relPath, err := filepath.Rel(cwd, absPath)
			if err != nil || strings.HasPrefix(relPath, "..") {
				return nil
			}

			baselinePath := filepath.Join(centralIncludesDir, relPath)

			liveBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if strings.Contains(string(liveBytes), "\x00") {
				return nil
			}

			baselineBytes, _ := os.ReadFile(baselinePath)

			if string(liveBytes) != string(baselineBytes) {
				xmlBuilder.WriteString(fmt.Sprintf("<file path=\"%s\" state=\"modified_since_last_turn\" type=\"full_text\">\n%s\n</file>\n", path, string(liveBytes)))
				modifiedCount++
				os.MkdirAll(filepath.Dir(baselinePath), 0755)
				os.WriteFile(baselinePath, liveBytes, 0644)
			}
			return nil
		})
	}

	if modifiedCount > 0 {
		return fmt.Sprintf("<system_context>\n%s</system_context>", strings.TrimSpace(xmlBuilder.String()))
	}
	return "-"
}

func sendAndRender(ctx context.Context, prompt string) {
	activeChat := GetActiveChat()

	fmt.Printf("\033[90mThinking...\033[0m\n\n")

	// 1. Compute the JIT Context Protocol
	contextStr := buildJITContextAndSync()

	apiPrompt := prompt
	if contextStr != "-" {
		apiPrompt = contextStr + "\n\n" + prompt
	}

	// 2. Dispatch to Gemini
	response, err := SendPrompt(ctx, apiPrompt)
	if err != nil {
		fmt.Printf("\033[31m[API Error]\033[0m %v\n", err)
		return
	}
	lastAPIResponse = response

	// 3. RAW TERMINAL OUTPUT
	fmt.Println(response)
	fmt.Println()

	// 4. METADATA EXTRACTION FOR SCHEMA
	now := time.Now().UTC()
	dateISO := now.Format("2006-01-02T15:04:05Z")
	filenameTime := now.Format("20060102T150405")
	year := now.Format("2006")
	month := now.Format("01")

	cwd, _ := os.Getwd()

	modelName := "gemini-2.5-pro"
	if ProjectViper.IsSet("model_name") {
		modelName = ProjectViper.GetString("model_name")
	}

	var includesBuilder strings.Builder
	if len(Project.IncludedData) == 0 {
		includesBuilder.WriteString("-\n")
	} else {
		for _, inc := range Project.IncludedData {
			includesBuilder.WriteString("- " + inc + "\n")
		}
	}
	includesStr := strings.TrimSpace(includesBuilder.String())

	// Subject & Slug Generation
	words := strings.Fields(prompt)
	limit := 5
	if len(words) < limit {
		limit = len(words)
	}
	subjectStr := strings.Join(words[:limit], " ")

	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := reg.ReplaceAllString(subjectStr, "-")
	slug = strings.ToLower(strings.Trim(slug, "-"))
	if len(slug) > 20 {
		slug = strings.TrimRight(slug[:20], "-")
	}
	if slug == "" {
		slug = "interaction"
	}

	// 5. BUILD THE EXACT DESIGN DOCUMENT SCHEMA
	record := fmt.Sprintf(`## AI Interaction
### Date
%s
### Chat
%s
### Source
ggo-cli / %s
### Includes
%s
### Current working directory
%s
### Subject
%s
### Context
%s
### Prompt
%s

### Response
%s

---
`, dateISO, activeChat, modelName, includesStr, cwd, subjectStr, contextStr, prompt, response)

	// 6. APPEND TO ACTIVE CHAT
	home, _ := os.UserHomeDir()
	chatPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats", "chat-"+activeChat)
	if !strings.HasSuffix(chatPath, ".md") {
		chatPath += ".md"
	}
	f, err := os.OpenFile(chatPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(record)
		f.Close()
	}

	// 7. WRITE TO /tmp/sources ARCHIVE
	sourcesDir := filepath.Join("/tmp", "sources", year, month)
	os.MkdirAll(sourcesDir, 0755)

	archiveFilename := fmt.Sprintf("%s-%s.md", filenameTime, slug)
	archivePath := filepath.Join(sourcesDir, archiveFilename)

	os.WriteFile(archivePath, []byte(record), 0644)

	// Visual confirmation in the REPL
	fmt.Printf("\033[90m[ggo] Interaction Schema Recorded: %s\033[0m\n", archivePath)
}
