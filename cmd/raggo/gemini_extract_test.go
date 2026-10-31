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

package raggo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiExtractPipeline(t *testing.T) {
	tempDir := t.TempDir()
	recordsDir := filepath.Join(tempDir, "records")
	ingestDir := filepath.Join(tempDir, "ingest", "gemini")

	err := os.MkdirAll(ingestDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create ingest dir: %v", err)
	}

	attachmentPath := filepath.Join(ingestDir, "system-architecture.pdf")
	os.WriteFile(attachmentPath, []byte("dummy pdf content"), 0644)

	mockJSON := `[
		{
			"header": "Gemini Apps",
			"title": "Prompted Write a massive Go function.",
			"time": "2026-04-20T14:30:00Z",
			"safeHtmlItem": [
				{
					"html": "<p>Here is your function:</p><pre><code>func a() {}\nfunc b() {}\nfunc c() {}\nfunc d() {}\nfunc e() {}\nfunc f() {}\nfunc g() {}\nfunc h() {}\nfunc i() {}\nfunc j() {}\nfunc k() {}\nfunc l() {}</code></pre>"
				}
			],
			"attachedFiles": ["system-architecture.pdf"]
		}
	]`

	jsonPath := filepath.Join(ingestDir, "MyActivity.json")
	err = os.WriteFile(jsonPath, []byte(mockJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock JSON: %v", err)
	}

	RunGeminiExtract(recordsDir, ingestDir, false)

	expectedYearMonth := filepath.Join("2026", "04")
	targetDir := filepath.Join(recordsDir, expectedYearMonth)

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Fatalf("JSON ledger directory was not created: %s", targetDir)
	}

	files, _ := os.ReadDir(targetDir)
	if len(files) != 1 {
		t.Fatalf("Expected exactly 1 file in JSON ledger, got %d", len(files))
	}

	content, _ := os.ReadFile(filepath.Join(targetDir, files[0].Name()))

	if !strings.Contains(string(content), "func l() {}") {
		t.Errorf("JSON ledger should contain the complete code block")
	}

	if !strings.Contains(string(content), "system-architecture.pdf") {
		t.Errorf("Failed to map attached file to the AiRecord")
	}
}
