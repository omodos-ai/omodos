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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/cmd/raggo"
	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
)

func TestChatRecordLifecycle(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()

	core.GlobalCfg.RecordsDir = filepath.Join(tempHome, "mock_records_tree")
	core.ProjectCfg.ProjectName = "test_project"
	core.GlobalSt.Projects = map[string]string{
		"test_project": filepath.Join(tempHome, "test_workspace"),
	}

	projDir := filepath.Join(tempHome, ".ggo", "ggoai-test_project")
	chatsDir := filepath.Join(projDir, "chats")
	err := os.MkdirAll(chatsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create isolated chats dir: %v", err)
	}

	expectedPrompt := "How well is this project structured?"
	expectedResponse := "The architecture is extremely robust and testable."

	// Create a dummy legacy .md file to test the extractor
	dummyMD := `## AI Interaction
### Date
2026-04-19T10:00:00Z
### Chat
legacy_md_chat
### Source
ggo-cli / gemini-2.5-pro
### Includes
-
### Current working directory
/Users/test
### Subject
test-subject
### Context
-
### Prompt
How well is this project structured?
### Response
The architecture is extremely robust and testable.
### State update
- Transaction Anchor: 20260419T100000Z
---
`
	mdPath := filepath.Join(chatsDir, "legacy_chat.md")
	os.WriteFile(mdPath, []byte(dummyMD), 0644)

	os.RemoveAll(core.GlobalCfg.RecordsDir)
	os.MkdirAll(core.GlobalCfg.RecordsDir, 0755)

	raggo.RunMdchatExtract(core.GlobalCfg.RecordsDir, chatsDir, false)

	var reconstructedPath string
	err = filepath.Walk(core.GlobalCfg.RecordsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			reconstructedPath = path
		}
		return nil
	})

	if err != nil || reconstructedPath == "" {
		t.Fatalf("mdchat extractor failed to reconstruct the JSON record from the local markdown")
	}

	reconBytes, _ := os.ReadFile(reconstructedPath)
	reconContent := string(reconBytes)

	if !strings.Contains(reconContent, expectedPrompt) {
		t.Errorf("Reconstructed JSON record is missing the prompt:\n%s", reconContent)
	}
	if !strings.Contains(reconContent, expectedResponse) {
		t.Errorf("Reconstructed JSON record is missing the response:\n%s", reconContent)
	}
}
