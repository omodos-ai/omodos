package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRolePersonaEngine(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	projName := "role_test_project"
	projVault := filepath.Join(tempHome, ".ggo", "ggoai-"+projName)
	os.MkdirAll(projVault, 0755)

	// Initialize a dummy git repo so ExecuteTransaction doesn't complain
	exec.Command("git", "-C", projVault, "init").Run()

	configPath := filepath.Join(tempHome, ".ggo", "config.yaml")
	configContent := "last_project: " + projName + "\nprojects:\n  " + projName + ": " + tempHome + "\n"
	os.WriteFile(configPath, []byte(configContent), 0644)

	Project.ProjectName = projName
	InitConfig()

	// 2. Hijack Stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 3. User Actions (Simulating REPL input)
	handleSet([]string{"set", "role", "math"})
	handleAddRole([]string{"add", "role", "hacker", "You are an elite cyber security penetration tester."})
	handleShowRoles()

	// 4. Restore Stdout & Read
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 5. Verification: Console Output
	if !strings.Contains(output, "Updated: role = math") {
		t.Errorf("Failed to confirm role update. Output:\n%s", output)
	}

	// The custom role should have been scanned from disk and added to the UI
	if !strings.Contains(output, "hacker") || !strings.Contains(output, "cyber security") {
		t.Errorf("'show roles' table missing the dynamically added 'hacker' role. Output:\n%s", output)
	}

	// 6. Verification: Physical Disk Extraction
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
