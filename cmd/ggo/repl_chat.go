// Copyright 2025-2026 Peter Braam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ggo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/chzyer/readline"
	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
)

func reviveLastResponse(chatName string) {
	chatDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "chats", chatName)
	files, err := os.ReadDir(chatDir)
	if err != nil || len(files) == 0 {
		lastAPIResponse = ""
		return
	}

	var jsonFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			jsonFiles = append(jsonFiles, f.Name())
		}
	}

	if len(jsonFiles) == 0 {
		lastAPIResponse = ""
		return
	}

	sort.Strings(jsonFiles)
	lastFile := filepath.Join(chatDir, jsonFiles[len(jsonFiles)-1])

	record, err := core.ReadRecord(lastFile)
	if err != nil || record.Output == nil {
		lastAPIResponse = ""
		return
	}

	lastAPIResponse = strings.TrimSpace(record.Output.Narrative)
}

func handleCC(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: cc <chat-name>")
		return
	}
	core.ActiveChatName = parts[1]

	core.ProjectViper.Set("active_chat", core.ActiveChatName)
	core.ProjectViper.WriteConfigAs(filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "project.yaml"))

	chatDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "chats", core.ActiveChatName)

	if _, err := os.Stat(chatDir); os.IsNotExist(err) {
		os.MkdirAll(chatDir, 0755)
		fmt.Printf("[✓] Created and activated chat: %s\n", core.ActiveChatName)
		lastAPIResponse = ""
	} else {
		fmt.Printf("[Info] Activated existing chat: %s\n", core.ActiveChatName)
		reviveLastResponse(core.ActiveChatName)
	}

	core.UpdateChatModel()
}

func handleP(ctx context.Context, l *readline.Instance) {
	fmt.Println("\n\033[35m=== Entering Chat Mode (Multi-line) ===\033[0m")
	for {
		prompt, ok := readMultiLine(l, "(Ctrl-D to send, Ctrl-C to exit Chat Mode)")
		if !ok {
			break
		}
		if prompt != "" {
			sendAndRender(ctx, prompt, false, l.Stdout())
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
		sendAndRender(ctx, finalPrompt, false, l.Stdout())
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

// BaselineSync holds deferred file writes for the transactional context update
type BaselineSync struct {
	Path string
	Data []byte
}

func buildJITContextAndSync() (string, []BaselineSync) {
	if len(core.ProjectSt.IncludedData) == 0 {
		return "-", nil
	}

	var xmlBuilder strings.Builder
	var pendingSyncs []BaselineSync
	modifiedCount := 0
	centralIncludesDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "includes")
	cwd, _ := os.Getwd()

	for _, targetPath := range core.ProjectSt.IncludedData {
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

				// Defer the physical disk write until we confirm the LLM received it
				pendingSyncs = append(pendingSyncs, BaselineSync{
					Path: baselinePath,
					Data: liveBytes,
				})
			}
			return nil
		})
	}

	if modifiedCount > 0 {
		return fmt.Sprintf("<system_context>\n%s</system_context>", strings.TrimSpace(xmlBuilder.String())), pendingSyncs
	}
	return "-", nil
}

func DispatchPrompt(ctx context.Context, prompt string, source string) (string, error) {
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Beginning DispatchPrompt for source: %s\n", source)
	activeChat := core.GetActiveChat()

	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Building JIT context...\n")
	contextStr, pendingSyncs := buildJITContextAndSync()
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] JIT context built. Length: %d\n", len(contextStr))

	apiPrompt := prompt
	if contextStr != "-" {
		apiPrompt = contextStr + "\n\n" + prompt
	}

	now := time.Now().UTC()
	dateISO := now.Format("2006-01-02T15:04:05Z")
	filenameTime := now.Format("20060102T150405")
	anchor := now.Format("20060102T150405Z")

	modelName := "gemini-2.5-pro"
	if core.ProjectViper.IsSet("model_name") {
		modelName = core.ProjectViper.GetString("model_name")
	}

	slug := core.MakeSafeSlug(prompt)
	chatDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "chats", activeChat)
	jsonFilename := fmt.Sprintf("%s-%s.json", filenameTime, slug)

	// ==========================================
	// STAGE 1: Write Incomplete Record Immediately
	// ==========================================
	record := &pb.AiRecord{
		Metadata: &pb.Metadata{
			TimestampUtc:      dateISO,
			ChatId:            activeChat,
			ModelName:         modelName,
			TransactionAnchor: anchor,
		},
		Input: &pb.UserInput{
			Narrative: apiPrompt, // Captures full prompt + context instantly
		},
	}
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Saving incomplete prompt to local ledger...\n")
	core.WriteRecord(chatDir, jsonFilename, record)

	// ==========================================
	// API EXECUTION
	// ==========================================
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Yielding to core.SendPrompt...\n")
	response, err := core.SendPrompt(ctx, apiPrompt)
	if err != nil {
		fmt.Printf("[DEBUG-TRACE] [DISPATCH] core.SendPrompt failed: %v\n", err)
		// Return without updating baseline, leaving desk state dirty for the next attempt.
		// The incomplete prompt safely remains on disk.
		return "", err
	}
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] core.SendPrompt succeeded. Response length: %d\n", len(response))

	lastAPIResponse = response
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Auto-copying to clipboard...\n")
	autoCopyToClipboard(response)

	// ==========================================
	// STAGE 2: Success - Overwrite Record & Commit Context
	// ==========================================
	fmt.Printf("[DEBUG-TRACE] [DISPATCH] API Success. Overwriting record and flushing context baselines...\n")
	record.Output = &pb.AIResponse{
		Narrative: response,
	}
	core.WriteRecord(chatDir, jsonFilename, record)

	// Flush the transactional baseline syncs to disk now that the LLM has them
	for _, sync := range pendingSyncs {
		os.MkdirAll(filepath.Dir(sync.Path), 0755)
		os.WriteFile(sync.Path, sync.Data, 0644)
	}

	cleanResp := strings.ReplaceAll(response, "\x00", "")
	cleanResp = strings.ReplaceAll(cleanResp, "\x1b", "")

	fmt.Printf("[DEBUG-TRACE] [DISPATCH] Returning to caller.\n")
	return cleanResp, nil
}

func sendAndRender(ctx context.Context, prompt string, fromPipe bool, stdout io.Writer) {
	fmt.Fprintf(stdout, "\033[90mThinking...\033[0m\n\n")

	source := "ggo-cli"
	if fromPipe {
		source = "ggo-emacs-pipe"
	}

	response, err := DispatchPrompt(ctx, prompt, source)
	if err != nil {
		fmt.Fprintf(stdout, "\033[31m[API Error]\033[0m %v\n", err)
		if ResponsePipe != nil {
			ResponsePipe.WriteString(fmt.Sprintf("[API Error] %v\n", err))
		}
		return
	}

	fmt.Fprintln(stdout, response)
	fmt.Fprintln(stdout)

	if ResponsePipe != nil {
		ResponsePipe.WriteString(response + "\n")
	}
}

func autoCopyToClipboard(response string) {
	re := regexp.MustCompile(`(?s)` + "```" + `[a-zA-Z0-9]*\n(.*?)\n` + "```")
	matches := re.FindAllStringSubmatch(response, -1)

	if len(matches) > 0 {
		var sb strings.Builder
		for i, m := range matches {
			snippet := sanitizeSnippet(m[1])
			sb.WriteString(snippet)
			if i < len(matches)-1 {
				sb.WriteString("\n\n")
			}
		}

		finalClipboardText := sb.String()
		if finalClipboardText != "" {
			err := clipboard.WriteAll(finalClipboardText)
			if err != nil {
				fmt.Printf("\033[33m[Warning] Failed to write snippets to clipboard: %v\033[0m\n", err)
			} else {
				fmt.Printf("\033[32m[✓] Auto-extracted %d snippet(s) to OS clipboard.\033[0m\n\n", len(matches))
			}
		}
	}
}

func sanitizeSnippet(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.ReplaceAll(text, "\x1b", "")
	return strings.TrimSpace(text)
}

