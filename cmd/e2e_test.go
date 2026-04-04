package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const e2eScript = `
# 1. Enroll the repository (Offline safe)
inc .

# 2. Trigger a deterministic transaction to capture the desk state perfectly!
set shell_cmd bash

# 3. List the timeline
gl
`

const rollbackScript = `
# Vault back to the set_shell_cmd transaction (Index 0)
gs 0
`

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command '%s %v' failed in %s: %v\nOutput: %s", name, args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// runOfflineGgo forces ggo to run purely locally by stripping the API key from its environment
func runOfflineGgo(t *testing.T, binPath, dir, script string) string {
	cmd := exec.Command(binPath)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(script)

	// Strip the GEMINI_API_KEY to ensure ultra-fast, offline test execution
	env := os.Environ()
	for i, v := range env {
		if strings.HasPrefix(v, "GEMINI_API_KEY=") {
			env[i] = "GEMINI_API_KEY="
		}
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ggo execution failed: %v\nOutput: %s", err, string(out))
	}
	return string(out)
}

func TestEndToEndTimeMachine(t *testing.T) {
	// 1. Compile the active ggo binary
	tempDir, err := os.MkdirTemp("", "ggo-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	binPath := filepath.Join(tempDir, "ggo-bin")
	buildOut, err := exec.Command("go", "build", "-o", binPath, "../main.go").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile ggo for E2E test: %v\n%s", err, string(buildOut))
	}

	// 2. Setup the Work Directory
	workDir := filepath.Join(tempDir, "workspace")
	os.MkdirAll(workDir, 0755)

	runCmd(t, workDir, "git", "init")
	runCmd(t, workDir, "git", "config", "user.email", "test@ggo.local")
	runCmd(t, workDir, "git", "config", "user.name", "E2E Tester")

	// Baseline Commit
	os.WriteFile(filepath.Join(workDir, "base.txt"), []byte("base"), 0644)
	runCmd(t, workDir, "git", "add", "base.txt")
	runCmd(t, workDir, "git", "commit", "-m", "init")

	// Set up the base for the file that will be unstaged
	os.WriteFile(filepath.Join(workDir, "unstaged.txt"), []byte("v1"), 0644)
	runCmd(t, workDir, "git", "add", "unstaged.txt")
	runCmd(t, workDir, "git", "commit", "-m", "add unstaged base")

	// --- Create The Complex Desk State ---
	// 1. Unstaged modification
	os.WriteFile(filepath.Join(workDir, "unstaged.txt"), []byte("v2-unstaged"), 0644)

	// 2. Staged new file
	os.WriteFile(filepath.Join(workDir, "staged.txt"), []byte("ready"), 0644)
	runCmd(t, workDir, "git", "add", "staged.txt")

	// 3. Initialize ggo non-interactively
	runOfflineGgo(t, binPath, workDir, "y\nE2ETestProject\n")

	// 4. Feed the first script to enroll the repo and capture state
	out := runOfflineGgo(t, binPath, workDir, e2eScript)
	t.Logf("\n--- ggo script execution ---\n%s\n----------------------------", out)

	// 5. Advance Time
	runCmd(t, workDir, "git", "commit", "-m", "committed the staged file")
	os.WriteFile(filepath.Join(workDir, "future.txt"), []byte("future"), 0644)
	runCmd(t, workDir, "git", "add", "future.txt")
	runCmd(t, workDir, "git", "commit", "-m", "future")

	// 6. Travel Back in Time!
	rbOut := runOfflineGgo(t, binPath, workDir, rollbackScript)
	t.Logf("\n--- ggo rollback execution ---\n%s\n------------------------------", rbOut)

	// 7. ASSERTIONS
	// A. Future file must be gone
	if _, err := os.Stat(filepath.Join(workDir, "future.txt")); !os.IsNotExist(err) {
		t.Fatalf("future.txt survived the rollback! Time machine failed.")
	}

	// B. Verify Staged state is flawless
	stagedStatus := runCmd(t, workDir, "git", "diff", "--cached", "--name-only")
	if !strings.Contains(stagedStatus, "staged.txt") {
		t.Fatalf("staged.txt was not perfectly restored to the index!")
	}

	// C. Verify Unstaged state is flawless
	unstagedStatus := runCmd(t, workDir, "git", "diff", "--name-only")
	if !strings.Contains(unstagedStatus, "unstaged.txt") {
		t.Fatalf("unstaged.txt modifications were lost!")
	}
}
