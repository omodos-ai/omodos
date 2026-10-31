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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
)

func TestRolePersonaEngine(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	os.Setenv("GGO_CONFIG_DIR", filepath.Join(tempHome, ".ggo"))
	defer os.Unsetenv("GGO_CONFIG_DIR")
	defer os.Setenv("HOME", origHome)
	origDir, _ := os.Getwd()
	os.Chdir(tempHome)
	defer os.Chdir(origDir)

	projName := "role_test_project"
	projVault := filepath.Join(tempHome, ".ggo", "ggoai-"+projName)
	os.MkdirAll(projVault, 0755)

	exec.Command("git", "-C", projVault, "init").Run()

	// FIX: Write state.json instead of config.yaml
	statePath := filepath.Join(tempHome, ".ggo", "state.json")
	stateContent := `{"last_project": "` + projName + `", "projects": {"` + projName + `": "` + tempHome + `"}}`
	os.WriteFile(statePath, []byte(stateContent), 0644)

	core.ProjectCfg.ProjectName = projName
	core.InitConfig("role_test_project")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleSet([]string{"set", "role", "math"})
	handleAddRole([]string{"add", "role", "hacker", "You are an elite cyber security penetration tester."})
	handleShowRoles()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Updated config: role = math") {
		t.Errorf("Failed to confirm role update. Output:\n%s", output)
	}
	if !strings.Contains(output, "hacker") || !strings.Contains(output, "cyber security") {
		t.Errorf("'show roles' table missing the dynamically added 'hacker' role. Output:\n%s", output)
	}

	rulesBytes, err := os.ReadFile(filepath.Join(projVault, "RULES.md"))
	if err != nil {
		t.Fatalf("ggo failed to dynamically extract RULES.md to disk: %v", err)
	}
	if !strings.Contains(string(rulesBytes), "Strict Schema") {
		t.Errorf("RULES.md is corrupted on disk.")
	}

	mathBytes, err := os.ReadFile(filepath.Join(projVault, "role-math.md"))
	if err != nil {
		t.Fatalf("ggo failed to dynamically extract role-math.md to disk: %v", err)
	}
	if !strings.Contains(string(mathBytes), "assisting a mathematics professor") {
		t.Errorf("role-math.md failed to write the correct fallback persona to disk.")
	}

	hackerBytes, err := os.ReadFile(filepath.Join(projVault, "role-hacker.md"))
	if err != nil {
		t.Fatalf("ggo failed to write the custom dynamic role to disk: %v", err)
	}
	if !strings.Contains(string(hackerBytes), "penetration tester") {
		t.Errorf("role-hacker.md was corrupted during write.")
	}
}
