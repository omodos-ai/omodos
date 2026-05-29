package ggo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
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

	os.Setenv("HOME", tempDir)

	os.MkdirAll(filepath.Join(tempDir, ".ggo", "ggoai-e2e_test"), 0755)

	// FIX: Write state.json instead of config.yaml to properly track the active project!
	statePath := filepath.Join(tempDir, ".ggo", "state.json")
	stateContent := `{"last_project": "e2e_test", "projects": {"e2e_test": "` + tempDir + `"}}`
	os.WriteFile(statePath, []byte(stateContent), 0644)

	os.WriteFile(filepath.Join(tempDir, ".ggo", "ggoai-e2e_test", "project.yaml"), []byte(""), 0644)

	exec.Command("git", "init").Run()
	os.WriteFile("main.go", []byte("package main\n"), 0644)
	exec.Command("git", "add", "main.go").Run()
	exec.Command("git", "commit", "-m", "Initial").Run()

	core.ProjectCfg = &pb.ProjectConfig{
		ProjectName: "e2e_test",
		CodeDirs:    []string{},
	}

	core.InitConfig()

	t.Log("--- ggo script execution ---")
	runScript("gr .\nset shell_cmd bash\ngl\nexit\n")

	os.WriteFile("future.txt", []byte("I am a bug"), 0644)

	t.Log("--- ggo rollback execution ---")
	runScript("gs 0\nexit\n")

	if _, err := os.Stat("future.txt"); err == nil {
		t.Fatalf("future.txt survived the rollback! Time machine failed.")
	}
}
