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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
)

func TestExtractPendingFiles(t *testing.T) {
	mockResponse := `
Sure, here are the files you requested.

<file path="src/main.go">
package main
func main() {
	println("hello")
}
</file>

And a config file:

<file path="config.yaml">
port: 8080
</file>
`

	files := extractPendingFiles(mockResponse)
	if len(files) != 2 {
		t.Fatalf("Expected 2 files extracted, got %d", len(files))
	}

	if files[0].Path != "src/main.go" {
		t.Errorf("Expected path src/main.go, got %s", files[0].Path)
	}
	if !strings.Contains(files[0].Content, "println(\"hello\")") {
		t.Errorf("Content mismatch for file 1: %s", files[0].Content)
	}

	if files[1].Path != "config.yaml" {
		t.Errorf("Expected path config.yaml, got %s", files[1].Path)
	}
	if !strings.Contains(files[1].Content, "port: 8080") {
		t.Errorf("Content mismatch for file 2: %s", files[1].Content)
	}
}

func TestFileSubsystemUnit(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()
	core.ProjectCfg.ProjectName = "file_subsystem_test"
	core.ProjectCfg.CodeDirs = []string{}

	projVault := filepath.Join(tempHome, ".ggo", "ggoai-file_subsystem_test")
	os.MkdirAll(projVault, 0755)

	// Create git repo so ExecuteTransaction succeeds
	core.ExecuteTransaction("init_test", func(a string, n time.Time) error { return nil })

	file1Path := filepath.Join(tempHome, "test_file_1.txt")
	file2Path := filepath.Join(tempHome, "nested", "test_file_2.go")

	// Inject a simulated AI response into the global volatile memory
	lastAPIResponse = fmt.Sprintf(`
<file path="%s">
Test Data 1
</file>
<file path="%s">
Test Data 2
</file>
`, file1Path, file2Path)

	// Test A: File List (fl)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleFL()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Pending AI File Edits") || !strings.Contains(output, "test_file_1.txt") {
		t.Errorf("'fl' command failed to list pending files. Output:\n%s", output)
	}

	// Test B: Targeted File Write (fw 0)
	handleFW([]string{"0"})

	if _, err := os.Stat(file1Path); os.IsNotExist(err) {
		t.Errorf("'fw 0' failed to write file 1 to disk.")
	}
	if _, err := os.Stat(file2Path); err == nil {
		t.Errorf("'fw 0' incorrectly wrote file 2 when it should have only targeted index 0.")
	}

	// Test C: File Merge (fm 1)
	core.ProjectViper.Set("diff_cmd", "diff -u") // Set a safe, non-blocking diff command
	handleFM([]string{"1"})

	proposedPath := file2Path + ".proposed"
	if _, err := os.Stat(proposedPath); os.IsNotExist(err) {
		t.Errorf("'fm 1' failed to generate the .proposed file for the merge workflow.")
	}

	// Test D: File Cancel (fc)
	handleFC()
	if lastAPIResponse != "" {
		t.Errorf("'fc' failed to clear the pending response queue.")
	}
}

func TestLiveFileWriteWorkflow(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		userHome, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(userHome, "secrets", "gemini-token")); os.IsNotExist(err) {
			t.Skip("Skipping live API test: No env var and no secrets file found.")
		}
	}

	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectViper = viper.New()
	core.GlobalViper = viper.New()
	core.ProjectCfg.ProjectName = "live_fw_test"
	core.ProjectCfg.CodeDirs = []string{}
	core.ActiveChatName = "live_chat"

	os.MkdirAll(filepath.Join(tempHome, ".ggo", "ggoai-live_fw_test", "chats"), 0755)

	ctx := context.Background()
	err := core.InitGemini(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize Gemini: %v", err)
	}

	file1Path := filepath.Join(tempHome, "live_fw_target_1.txt")
	file2Path := filepath.Join(tempHome, "live_fw_target_2.txt")

	// 2. Action: Prompt the AI to generate files
	prompt := fmt.Sprintf(`I need you to generate two text files using your strict XML <file> schema.
Do not use markdown codeblocks inside the XML.
1. Path: "%s", Content: "Live Target 1"
2. Path: "%s", Content: "Live Target 2"`, file1Path, file2Path)

	lastAPIResponse = ""
	sendAndRender(ctx, prompt, false, os.Stdout)
	if lastAPIResponse == "" {
		t.Skip("Skipping due to Google API 503 error")
	}

	// 3. Action: Apply all files (fw with no args)
	handleFW([]string{})

	// 4. Verification
	content1, err := os.ReadFile(file1Path)
	if err != nil {
		t.Fatalf("Failed to read live generated file 1: %v", err)
	}
	if !strings.Contains(string(content1), "Live Target 1") {
		t.Errorf("Live generated file 1 content mismatch. Got: %q", string(content1))
	}

	content2, err := os.ReadFile(file2Path)
	if err != nil {
		t.Fatalf("Failed to read live generated file 2: %v", err)
	}
	if !strings.Contains(string(content2), "Live Target 2") {
		t.Errorf("Live generated file 2 content mismatch. Got: %q", string(content2))
	}
}
