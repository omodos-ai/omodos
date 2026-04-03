package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var (
	geminiClient *genai.Client
	chatSession  *genai.ChatSession
)

// InitGemini boots up the client and starts a stateful chat session.
func InitGemini(ctx context.Context) error {
	// 1. Resolve the API Key
	apiKey := ""
	if Global.APIKeyPath != "" {
		keyBytes, err := os.ReadFile(Global.APIKeyPath)
		if err == nil {
			apiKey = strings.TrimSpace(string(keyBytes))
		}
	}

	// Fallback to environment variable if file is missing or empty
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	if apiKey == "" {
		return fmt.Errorf("no API key found in %s or GEMINI_API_KEY env var", Global.APIKeyPath)
	}

	// 2. Initialize the Client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	geminiClient = client

	// 3. Initialize the Model and Stateful Chat
	model := client.GenerativeModel("gemini-2.5-pro")
	// You can adjust temperature here if needed: model.SetTemperature(0.2)
	
	chatSession = model.StartChat()

	return nil
}

// SendPrompt sends a message to the active chat session and returns the response.
func SendPrompt(ctx context.Context, prompt string) (string, error) {
	if chatSession == nil {
		return "", fmt.Errorf("chat session is not initialized")
	}

	resp, err := chatSession.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	return extractText(resp), nil
}

// extractText safely pulls the string content out of the SDK's response payload
func extractText(resp *genai.GenerateContentResponse) string {
	var sb strings.Builder
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if text, ok := part.(genai.Text); ok {
					sb.WriteString(string(text))
				}
			}
		}
	}
	return sb.String()
}
