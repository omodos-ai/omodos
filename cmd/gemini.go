package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var chatSession *genai.ChatSession

func InitGemini(ctx context.Context) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return err
	}
	
	Runtime.GeminiClient = client
	UpdateChatModel() // Initialize the session
	
	return nil
}

// UpdateChatModel cleanly swaps the engine while preserving conversational history
func UpdateChatModel() {
	if Runtime.GeminiClient == nil { return }
	
	model := Runtime.GeminiClient.GenerativeModel(Project.ModelName)
	
	var oldHistory []*genai.Content
	if chatSession != nil {
		oldHistory = chatSession.History
	}
	
	chatSession = model.StartChat()
	if oldHistory != nil {
		chatSession.History = oldHistory
	}
}

func SendPrompt(ctx context.Context, prompt string) (string, error) {
	if chatSession == nil {
		return "", fmt.Errorf("chat session not initialized")
	}

	resp, err := chatSession.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}
	return sb.String(), nil
}

func FetchAndListModels(ctx context.Context) {
	if Runtime.GeminiClient == nil {
		fmt.Println("[Error] Gemini API not initialized.")
		return
	}

	fmt.Println("Fetching available models...")
	iter := Runtime.GeminiClient.ListModels(ctx)
	
	View.ModelList = []string{}
	idx := 1

	fmt.Println("\n--- Available Gemini Models ---")
	for {
		m, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("[Error] Failed to fetch models: %v\n", err)
			return
		}

		cleanName := strings.TrimPrefix(m.Name, "models/")
		View.ModelList = append(View.ModelList, cleanName)

		if cleanName == Project.ModelName {
			fmt.Printf("[\033[32m%d\033[0m] \033[36m%s\033[0m (Active)\n", idx, cleanName)
		} else {
			fmt.Printf("[\033[32m%d\033[0m] %s\n", idx, cleanName)
		}
		idx++
	}
	fmt.Println()
}
