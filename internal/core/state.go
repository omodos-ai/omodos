package core

import (
	"google.golang.org/genai"
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

// --- Chat State ---

var ActiveChatName string

func GetActiveChat() string {
	if ActiveChatName != "" {
		return ActiveChatName
	}
	if ProjectViper != nil && ProjectViper.IsSet("active_chat") {
		ActiveChatName = ProjectViper.GetString("active_chat")
		return ActiveChatName
	}
	return "default"
}
