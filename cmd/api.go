package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

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
		pIdx := strings.Index(block, "### Prompt\n")
		rIdx := strings.Index(block, "### Response\n")

		if pIdx != -1 && rIdx != -1 && pIdx < rIdx {
			prompt := strings.TrimSpace(block[pIdx+len("### Prompt\n") : rIdx])

			respRaw := block[rIdx+len("### Response\n"):]
			lastDash := strings.LastIndex(respRaw, "---")
			if lastDash != -1 {
				respRaw = respRaw[:lastDash]
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

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: string(systemPromptBytes)},
			},
		},
	}

	// Read the active chat file and hydrate the session memory
	activeChatName := GetActiveChat()
	chatPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "chats", "chat-"+activeChatName+".md")
	history := parseChatHistory(chatPath)

	var errChat error
	chatSession, errChat = aiClient.Chats.Create(context.Background(), modelName, config, history)
	if errChat != nil {
		fmt.Printf("[Warning] Failed to initialize chat session: %v\n", errChat)
	}
}

func SendPrompt(ctx context.Context, prompt string) (string, error) {
	if chatSession == nil {
		return "", fmt.Errorf("AI session not initialized")
	}

	Runtime.IsThinking = true
	defer func() { Runtime.IsThinking = false }()

	resp, err := chatSession.SendMessage(ctx, genai.Part{Text: prompt})
	if err != nil {
		return "", err
	}

	return resp.Text(), nil
}

func FetchAndListModels(ctx context.Context) {
	if aiClient == nil {
		fmt.Println("[Error] Gemini API not initialized.")
		return
	}

	fmt.Println("Fetching available models...")

	// aiClient.Models.List returns exactly 2 values in the modern SDK
	resp, err := aiClient.Models.List(ctx, nil)
	if err != nil {
		fmt.Printf("[Error] Failed to fetch models: %v\n", err)
		return
	}

	View.ModelList = []string{}

	// Robust reflection to extract the models safely, protecting against undocumented SDK structure shifts
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

		// Fallback for iterator struct with .Next()
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

	// Ultimate safety net
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
