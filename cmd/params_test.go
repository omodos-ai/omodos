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

func TestModelParameterTuning(t *testing.T) {
	// 1. Sandbox setup
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	projName := "param_test_project"
	projDir := filepath.Join(tempHome, ".ggo", "ggoai-"+projName)
	os.MkdirAll(projDir, 0755)

	// Initialize a dummy git repo so ExecuteTransaction doesn't complain
	exec.Command("git", "-C", projDir, "init").Run()

	// Mock global config so InitConfig can resolve the active project
	configPath := filepath.Join(tempHome, ".ggo", "config.yaml")
	configContent := "last_project: " + projName + "\nprojects:\n  " + projName + ": " + tempHome + "\n"
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Ensure Viper knows what project we are in
	Project.ProjectName = projName
	InitConfig()

	// 2. Hijack Stdout to capture CLI output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 3. User Actions (Simulating REPL input)
	handleSet([]string{"set", "temperature", "0.85"})
	handleSet([]string{"set", "top_p", "0.65"})
	handleSet([]string{"set", "safety_level", "low"})
	
	// Print the params table
	handleShowParams()

	// 4. Restore Stdout & Read
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 5. Verification: Console Output
	if !strings.Contains(output, "Updated: temperature = 0.85") {
		t.Errorf("Failed to confirm temperature update. Output:\n%s", output)
	}
	if !strings.Contains(output, "0.85") {
		t.Errorf("'show params' table missing temperature 0.85. Output:\n%s", output)
	}
	if !strings.Contains(output, "0.65") {
		t.Errorf("'show params' table missing top_p 0.65. Output:\n%s", output)
	}
	if !strings.Contains(output, "low") {
		t.Errorf("'show params' table missing safety_level 'low'. Output:\n%s", output)
	}

	// 6. Verification: Physical Disk Persistence
	yamlBytes, err := os.ReadFile(filepath.Join(projDir, "project.yaml"))
	if err != nil {
		t.Fatalf("Failed to read project.yaml: %v", err)
	}
	yamlStr := string(yamlBytes)

	if !strings.Contains(yamlStr, "temperature: 0.85") {
		t.Errorf("project.yaml failed to save temperature. Got:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "top_p: 0.65") {
		t.Errorf("project.yaml failed to save top_p. Got:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "safety_level: low") {
		t.Errorf("project.yaml failed to save safety_level. Got:\n%s", yamlStr)
	}
}
