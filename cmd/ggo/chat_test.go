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
	"sort"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/viper"
)

func TestLiveEndToEndChatAndContext(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		userHome, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(userHome, "secrets", "gemini-token")); os.IsNotExist(err) {
			t.Skip("Skipping live API test: No env var and no secrets file found.")
		}
	}

	tempDir, _ := filepath.EvalSymlinks(t.TempDir())

	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	core.GlobalCfg = &pb.GlobalConfig{}
	core.GlobalSt = &pb.GlobalState{Projects: make(map[string]string)}
	core.ProjectCfg = &pb.ProjectConfig{}
	core.ProjectSt = &pb.ProjectState{}

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()

	origConfigDir := os.Getenv("GGO_CONFIG_DIR")
	os.Setenv("GGO_CONFIG_DIR", filepath.Join(tempDir, ".ggo"))
	defer func() {
		if origConfigDir == "" {
			os.Unsetenv("GGO_CONFIG_DIR")
		} else {
			os.Setenv("GGO_CONFIG_DIR", origConfigDir)
		}
	}()

	core.ProjectCfg.ProjectName = "live_chat_test"
	os.MkdirAll(filepath.Join(tempDir, ".ggo", "ggoai-live_chat_test", "chats"), 0755)

	testFile := filepath.Join(tempDir, "system_config.txt")

	// --- Stage 1: Baseline Creation ---
	os.WriteFile(testFile, []byte("STATUS=OFFLINE\nVERSION=1.0"), 0644)

	lastAPIResponse = "" // strict reset
	script1 := "inc " + testFile + "\ncc live_test_chat\nprompt What is the current STATUS in system_config.txt? Answer with just the value.\nexit\n"
	runScript(script1)

	chatDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "chats", "live_test_chat")
	files, err := os.ReadDir(chatDir)
	if err != nil || len(files) == 0 {
		t.Fatalf("Failed to find generated chat directory or files: %v", err)
	}

	var jsonFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			jsonFiles = append(jsonFiles, f.Name())
		}
	}
	sort.Strings(jsonFiles)
	
	record1, _ := core.ReadRecord(filepath.Join(chatDir, jsonFiles[0]))

	if !strings.Contains(strings.ToUpper(record1.Output.Narrative), "OFFLINE") {
		t.Fatalf("Gemini failed to read the baseline context. Expected 'OFFLINE' in response. Got:\n%s", record1.Output.Narrative)
	}

	// --- Stage 2: The JIT Full-State Modification ---
	os.WriteFile(testFile, []byte("STATUS=ONLINE\nVERSION=2.0"), 0644)

	lastAPIResponse = "" // strict reset
	script2 := "cc live_test_chat\nprompt I just updated the config on disk. What is the STATUS now? Answer with just the value.\nexit\n"
	runScript(script2)

	files, _ = os.ReadDir(chatDir)
	jsonFiles = nil
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			jsonFiles = append(jsonFiles, f.Name())
		}
	}
	sort.Strings(jsonFiles)

	if len(jsonFiles) < 2 {
		t.Fatalf("Expected at least two JSON interaction files.")
	}

	record2, _ := core.ReadRecord(filepath.Join(chatDir, jsonFiles[len(jsonFiles)-1]))

	if !strings.Contains(record2.Input.Narrative, "<system_context>") {
		t.Fatalf("Expected XML <system_context> injection, but it was missing in the logged prompt.")
	}

	if !strings.Contains(strings.ToUpper(record2.Output.Narrative), "ONLINE") {
		t.Fatalf("Gemini failed to comprehend the JIT XML update. Expected 'ONLINE'. Response block:\n%s", record2.Output.Narrative)
	}
}
