package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"google.golang.org/genai"
)

//go:embed scripts/GEMINI.md
var defaultGeminiMD []byte

var aiClient *genai.Client
var chatSession *genai.Chat

func InitGemini(ctx context.Context) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	var err error
	aiClient, err = genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return err
	}

	UpdateChatModel()
	return nil
}

// parseChatHistory reads the markdown ledger and extracts past interactions to hydrate the API.
func parseChatHistory(chatPath string) []*genai.Content {
	var history []*genai.Content
	contentBytes, err := os.ReadFile(chatPath)
	if err != nil {
		return history // Return empty history if file doesn't exist yet
	}

	blocks := strings.Split(string(contentBytes), "## AI Interaction")
	for _, block := range blocks {
		cIdx := strings.Index(block, "### Context\n")
		pIdx := strings.Index(block, "### Prompt\n")
		rIdx := strings.Index(block, "### Response\n")

		if pIdx != -1 && rIdx != -1 && pIdx < rIdx {
			prompt := strings.TrimSpace(block[pIdx+len("### Prompt\n") : rIdx])

			// Re-hydrate the XML JIT context if it was recorded in this turn
			if cIdx != -1 && cIdx < pIdx {
				ctxStr := strings.TrimSpace(block[cIdx+len("### Context\n") : pIdx])
				if ctxStr != "-" && ctxStr != "" {
					prompt = ctxStr + "\n\n" + prompt
				}
			}

			respRaw := block[rIdx+len("### Response\n"):]

			// Safely ignore the state update block and trailing dashes
			sIdx := strings.Index(respRaw, "### State update\n")
			if sIdx != -1 {
				respRaw = respRaw[:sIdx]
			} else {
				lastDash := strings.LastIndex(respRaw, "---")
				if lastDash != -1 {
					respRaw = respRaw[:lastDash]
				}
			}
			response := strings.TrimSpace(respRaw)

			if prompt != "" && response != "" {
				history = append(history, &genai.Content{
					Role:  "user",
					Parts: []*genai.Part{{Text: prompt}},
				})
				history = append(history, &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: response}},
				})
			}
		}
	}
	return history
}

func UpdateChatModel() {
	if aiClient == nil {
		return
	}

	modelName := "gemini-2.5-pro" // Default
	if ProjectViper.IsSet("model_name") {
		modelName = ProjectViper.GetString("model_name")
	}

	home, _ := os.UserHomeDir()
	systemPromptPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "GEMINI.md")
	systemPromptBytes, err := os.ReadFile(systemPromptPath)

	if os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(systemPromptPath), 0755)
		os.WriteFile(systemPromptPath, defaultGeminiMD, 0644)
		systemPromptBytes = defaultGeminiMD
	}

	// Inject the Project Baseline into the System Instruction
	var sysPromptBuilder strings.Builder
	sysPromptBuilder.Write(systemPromptBytes)
	sysPromptBuilder.WriteString("\n\n### Project Baseline Context\n")
	sysPromptBuilder.WriteString("The following files represent the current read-only baseline state of the project. Use this as your primary knowledge base for the user's environment:\n\n")

	centralIncludesDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "includes")
	cwd, _ := os.Getwd()

	hasBaseline := false
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
			baselineBytes, err := os.ReadFile(baselinePath)
			if err == nil {
				sysPromptBuilder.WriteString(fmt.Sprintf("<file path=\"%s\">\n%s\n</file>\n\n", path, string(baselineBytes)))
				hasBaseline = true
			}
			return nil
		})
	}

	if !hasBaseline {
		sysPromptBuilder.WriteString("No files currently included.\n")
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: sysPromptBuilder.String()},
			},
		},
	}

	// Read the active chat file and hydrate the session memory
	activeChatName := GetActiveChat()
	chatPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats", activeChatName+".md")
	history := parseChatHistory(chatPath)

	var errChat error
	chatSession, errChat = aiClient.Chats.Create(context.Background(), modelName, config, history)
	if errChat != nil {
		fmt.Printf("[Warning] Failed to initialize chat session: %v\n", errChat)
	}
}

// SendPrompt is a variable so it can be safely mocked in the test suite
var SendPrompt = func(ctx context.Context, prompt string) (string, error) {
	if chatSession == nil {
		return "", fmt.Errorf("AI session not initialized")
	}

	Runtime.IsThinking = true
	defer func() { Runtime.IsThinking = false }()

	resp, err := chatSession.SendMessage(ctx, genai.Part{Text: prompt})
	if err != nil {
		return "", err
	}

	// --- NEW: Cost & Token Logging Engine ---
	go func(r *genai.GenerateContentResponse) {
		if r == nil || r.UsageMetadata == nil {
			return
		}

		inputTokens := r.UsageMetadata.PromptTokenCount
		outputTokens := r.UsageMetadata.CandidatesTokenCount
		totalTokens := r.UsageMetadata.TotalTokenCount

		// Pricing as of April 2026 for Gemini 1.5/2.5 Pro (under 128k context)
		// $1.25 per 1M input tokens, $5.00 per 1M output tokens
		inputCost := float64(inputTokens) * (1.25 / 1000000.0)
		outputCost := float64(outputTokens) * (5.00 / 1000000.0)
		totalCost := inputCost + outputCost

		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
		logPath := filepath.Join(logDir, "cost.csv")

		// Write CSV Header if file doesn't exist
		if _, statErr := os.Stat(logPath); os.IsNotExist(statErr) {
			os.WriteFile(logPath, []byte("Timestamp,Chat,InputTokens,OutputTokens,TotalTokens,EstCostUSD\n"), 0644)
		}

		now := time.Now().UTC().Format(time.RFC3339)
		activeChat := GetActiveChat()

		logLine := fmt.Sprintf("%s,%s,%d,%d,%d,$%.6f\n",
			now, activeChat, inputTokens, outputTokens, totalTokens, totalCost)

		f, fileErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fileErr == nil {
			f.WriteString(logLine)
			f.Close()
		}
	}(resp)
	// ----------------------------------------

	return resp.Text(), nil
}

func FetchAndListModels(ctx context.Context) {
	if aiClient == nil {
		fmt.Println("[Error] Gemini API not initialized.")
		return
	}

	fmt.Println("Fetching available models...")

	resp, err := aiClient.Models.List(ctx, nil)
	if err != nil {
		fmt.Printf("[Error] Failed to fetch models: %v\n", err)
		return
	}

	View.ModelList = []string{}

	val := reflect.ValueOf(resp)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	extractNames := func(v reflect.Value) {
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				m := v.Index(i)
				if m.Kind() == reflect.Ptr {
					m = m.Elem()
				}
				if m.Kind() == reflect.Struct {
					nameField := m.FieldByName("Name")
					if nameField.IsValid() && nameField.Kind() == reflect.String {
						name := strings.TrimPrefix(nameField.String(), "models/")
						View.ModelList = append(View.ModelList, name)
					}
				}
			}
		}
	}

	if val.Kind() == reflect.Slice {
		extractNames(val)
	} else if val.Kind() == reflect.Struct {
		found := false
		for _, fName := range []string{"Models", "Items", "Page"} {
			f := val.FieldByName(fName)
			if f.IsValid() && f.Kind() == reflect.Slice {
				extractNames(f)
				found = true
				break
			}
		}

		if !found {
			nextMethod := reflect.ValueOf(resp).MethodByName("Next")
			if nextMethod.IsValid() {
				for {
					res := nextMethod.Call(nil)
					if len(res) >= 1 {
						m := res[0]
						if len(res) == 2 && !res[1].IsNil() {
							break
						}
						if m.Kind() == reflect.Ptr {
							m = m.Elem()
						}
						if m.Kind() == reflect.Struct {
							nameField := m.FieldByName("Name")
							if nameField.IsValid() && nameField.Kind() == reflect.String {
								name := strings.TrimPrefix(nameField.String(), "models/")
								View.ModelList = append(View.ModelList, name)
							}
						} else {
							break
						}
					} else {
						break
					}
				}
			}
		}
	}

	if len(View.ModelList) == 0 {
		View.ModelList = []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-pro-exp"}
	}

	fmt.Println("\n--- Available Gemini Models ---")
	activeModel := "gemini-2.5-pro"
	if ProjectViper.IsSet("model_name") {
		activeModel = ProjectViper.GetString("model_name")
	}

	for idx, m := range View.ModelList {
		if m == activeModel {
			fmt.Printf("[\033[32m%d\033[0m] \033[36m%s\033[0m (Active)\n", idx+1, m)
		} else {
			fmt.Printf("[\033[32m%d\033[0m] %s\n", idx+1, m)
		}
	}
	fmt.Println()
}
