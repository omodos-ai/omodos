package cmd

import (
	"github.com/google/generative-ai-go/genai"
)

// GlobalRuntime holds persistent system state (clients, physical paths)
type GlobalRuntime struct {
	CWD          string
	GeminiClient *genai.Client
	IsThinking   bool // Tracks if an API call is active
}

// CommandView holds volatile UI state for the REPL (numbered lists)
type CommandView struct {
	ModelList []string
}

var (
	Runtime GlobalRuntime
	View    CommandView
)
