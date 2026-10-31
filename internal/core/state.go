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
