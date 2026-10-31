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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
)

func TestHandleShowCosts_ValidData(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectCfg.ProjectName = "cost_test_project"
	logDir := filepath.Join(tempHome, ".ggo", "ggoai-cost_test_project")
	os.MkdirAll(logDir, 0755)

	// 2. Create mock cost.csv with known mathematical constants
	csvContent := `Timestamp,Chat,InputTokens,OutputTokens,TotalTokens,EstCostUSD
2026-04-19T10:00:00Z,default,1000,500,1500,$0.003750
2026-04-19T10:05:00Z,default,2000,1000,3000,$0.007500
`
	logPath := filepath.Join(logDir, "cost.csv")
	os.WriteFile(logPath, []byte(csvContent), 0644)

	// 3. Hijack Stdout to capture the CLI output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 4. Action
	handleShowCosts([]string{"show", "costs", "all"})

	// 5. Restore Stdout & Read
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 6. Verification (The Math)
	if !strings.Contains(output, "TOTAL") {
		t.Fatalf("Expected 'TOTAL' row in output, got:\n%s", output)
	}

	// Input: 1000 + 2000 = 3000
	if !strings.Contains(output, "3000") {
		t.Errorf("Failed to sum Input Tokens correctly. Expected 3000, got:\n%s", output)
	}

	// Output: 500 + 1000 = 1500
	if !strings.Contains(output, "1500") {
		t.Errorf("Failed to sum Output Tokens correctly. Expected 1500, got:\n%s", output)
	}

	// Total Tokens: 1500 + 3000 = 4500
	if !strings.Contains(output, "4500") {
		t.Errorf("Failed to sum Total Tokens correctly. Expected 4500, got:\n%s", output)
	}

	// USD Cost: 0.003750 + 0.007500 = 0.011250
	if !strings.Contains(output, "0.011250") {
		t.Errorf("Failed to calculate USD Cost correctly. Expected $0.011250, got:\n%s", output)
	}
}

func TestHandleShowCosts_NoData(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	core.ProjectCfg.ProjectName = "empty_cost_project"

	// 2. Hijack Stdout (Intentionally NOT creating the log directory or CSV)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 3. Action
	handleShowCosts([]string{"show", "costs", "all"})

	// 4. Restore Stdout & Read
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 5. Verification (Graceful Fallback)
	if !strings.Contains(output, "No cost data recorded yet") {
		t.Errorf("Expected graceful missing file message, got:\n%s", output)
	}
}
