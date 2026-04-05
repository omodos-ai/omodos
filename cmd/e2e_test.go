package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func runScript(script string) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(script))
		w.Close()
	}()
	RunHeadless(context.Background())
	os.Stdin = oldStdin
}

func TestEndToEndTimeMachine(t *testing.T) {
	// FIX: Save and restore the CWD so rsync doesn't crash in subsequent tests
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tempDir, err := os.MkdirTemp("", "ggo-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	os.Chdir(tempDir)

	// Explicitly guarantee Viper is not nil before anything else runs
	ProjectViper = viper.New()
	GlobalViper = viper.New()

	// Stub the config files so InitConfig doesn't abort early
	os.MkdirAll(filepath.Join(tempDir, ".ggo", "ggoai-e2e_test"), 0755)
	os.WriteFile(filepath.Join(tempDir, ".ggo", "global.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tempDir, ".ggo", "ggoai-e2e_test", "project.yaml"), []byte(""), 0644)

	exec.Command("git", "init").Run()
	os.WriteFile("main.go", []byte("package main\n"), 0644)
	exec.Command("git", "add", "main.go").Run()
	exec.Command("git", "commit", "-m", "Initial").Run()

	Project = ProjectConfig{
		ProjectName: "e2e_test",
		CodeDirs:    []string{},
	}
	InitConfig()

	t.Log("--- ggo script execution ---")
	runScript("gr .\nset shell_cmd bash\ngl\nexit\n")

	os.WriteFile("future.txt", []byte("I am a bug"), 0644)

	t.Log("--- ggo rollback execution ---")
	runScript("gs 0\nexit\n")

	if _, err := os.Stat("future.txt"); err == nil {
		t.Fatalf("future.txt survived the rollback! Time machine failed.")
	}
}
