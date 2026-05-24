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
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	projName := "param_test_project"
	projDir := filepath.Join(tempHome, ".ggo", "ggoai-"+projName)
	os.MkdirAll(projDir, 0755)

	exec.Command("git", "-C", projDir, "init").Run()

	// FIX: Write state.json instead of config.yaml
	statePath := filepath.Join(tempHome, ".ggo", "state.json")
	stateContent := `{"last_project": "` + projName + `", "projects": {"` + projName + `": "` + tempHome + `"}}`
	os.WriteFile(statePath, []byte(stateContent), 0644)

	ProjectCfg.ProjectName = projName
	InitConfig()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleSet([]string{"set", "temperature", "0.85"})
	handleSet([]string{"set", "top_p", "0.65"})
	handleSet([]string{"set", "safety_level", "low"})

	handleShowParams()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

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
