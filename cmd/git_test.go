package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit is a test helper to execute git commands and return output cleanly
func runGit(t *testing.T, dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\nOutput: %s", args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestGitsyncThorough(t *testing.T) {
	// 1. Setup Temporary Directories
	tempDir, err := os.MkdirTemp("", "ggo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	subOriginDir := filepath.Join(tempDir, "submodule-origin")
	workDir := filepath.Join(tempDir, "working")
	shadowDir := filepath.Join(tempDir, "shadow")

	os.MkdirAll(subOriginDir, 0755)
	os.MkdirAll(workDir, 0755)

	// 2. Initialize Submodule Origin Repo
	runGit(t, subOriginDir, "init")
	runGit(t, subOriginDir, "config", "user.email", "test@ggo.local")
	runGit(t, subOriginDir, "config", "user.name", "Test User")
	os.WriteFile(filepath.Join(subOriginDir, "sub.txt"), []byte("submodule data"), 0644)
	runGit(t, subOriginDir, "add", "sub.txt")
	runGit(t, subOriginDir, "commit", "-m", "Initial submodule commit")

	// 3. Initialize Working Repository (The Foundation)
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.email", "test@ggo.local")
	runGit(t, workDir, "config", "user.name", "Test User")

	os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("v1\n"), 0644)
	os.WriteFile(filepath.Join(workDir, "stash-target.txt"), []byte("baseline\n"), 0644)
	runGit(t, workDir, "add", "tracked.txt", "stash-target.txt")
	runGit(t, workDir, "commit", "-m", "Initial commit")

	// Add the Submodule (Using protocol.file.allow to permit local path cloning in newer Git versions)
	runGit(t, workDir, "-c", "protocol.file.allow=always", "submodule", "add", subOriginDir, "mysub")
	runGit(t, workDir, "commit", "-m", "Added submodule")

	// 4. Create Complex Desk State (Stash, Partial Stage, Untracked)

	// A. Create a Stash
	os.WriteFile(filepath.Join(workDir, "stash-target.txt"), []byte("baseline\nstashed-work\n"), 0644)
	runGit(t, workDir, "stash", "push", "-m", "test-stash")

	// B. Create an Untracked File
	os.WriteFile(filepath.Join(workDir, "uncommitted.txt"), []byte("AI prompt context"), 0644)

	// C. Create a Partial Stage
	os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("v1\npartially-staged-line\n"), 0644)
	runGit(t, workDir, "add", "tracked.txt")                                                                        // Stage it
	os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("v1\npartially-staged-line\nunstaged-line\n"), 0644) // Modify again

	// 5. Capture Phase (Working -> Shadow)
	anchor := "20260404T120000Z"
	err = Gitsync(SyncParams{
		Direction: "WorkingToShadow",
		WorkDir:   workDir,
		ShadowDir: shadowDir,
		Anchor:    anchor,
		Phase:     "before",
		Message:   "thorough test transaction",
	})
	if err != nil {
		t.Fatalf("Gitsync WorkingToShadow failed: %v", err)
	}

	// 6. Verify Shadow Vault Backups
	// Verify Namespace Secret
	refPath := "refs/ggo/txn/" + anchor + "-before"
	shadowHash := runGit(t, shadowDir, "rev-parse", refPath)
	if len(shadowHash) != 40 {
		t.Fatalf("Expected 40-char hash in custom namespace, got: %s", shadowHash)
	}

	// Verify Stash Disaster Recovery Backup
	shadowStash := runGit(t, shadowDir, "rev-parse", "refs/stash")
	workStash := runGit(t, workDir, "rev-parse", "refs/stash")
	if shadowStash != workStash || len(shadowStash) != 40 {
		t.Fatalf("Stash was not properly backed up to the Shadow Vault. Work: %s, Shadow: %s", workStash, shadowStash)
	}

	// 7. Advance the Timeline (Simulate user moving forward)
	runGit(t, workDir, "checkout", "-b", "feature-branch")
	runGit(t, workDir, "add", "uncommitted.txt", "tracked.txt")
	runGit(t, workDir, "commit", "-m", "User committed their own work")

	os.WriteFile(filepath.Join(workDir, "future.txt"), []byte("This is from the future"), 0644)
	runGit(t, workDir, "add", "future.txt")
	runGit(t, workDir, "commit", "-m", "Future commit")

	// 8. Restore Phase (Shadow -> Working)
	err = Gitsync(SyncParams{
		Direction: "ShadowToWorking",
		WorkDir:   workDir,
		ShadowDir: shadowDir,
		Anchor:    anchor,
		Phase:     "before",
	})
	if err != nil {
		t.Fatalf("Gitsync ShadowToWorking failed: %v", err)
	}

	// 9. Verify Perfect Mathematical Restoration

	// A. Detached HEAD
	branch := runGit(t, workDir, "branch", "--show-current")
	if branch != "" {
		t.Fatalf("Expected detached HEAD, but we are on branch: %s", branch)
	}

	// B. Future file erased
	if _, err := os.Stat(filepath.Join(workDir, "future.txt")); !os.IsNotExist(err) {
		t.Fatalf("future.txt should not exist in the past state!")
	}

	// C. Untracked file perfectly restored
	uncommittedData, _ := os.ReadFile(filepath.Join(workDir, "uncommitted.txt"))
	if string(uncommittedData) != "AI prompt context" {
		t.Fatalf("uncommitted.txt was not perfectly restored")
	}

	// D. Submodule Survival Check
	// The rsync --delete should NOT have deleted the submodule's .git file
	subGitStat, err := os.Stat(filepath.Join(workDir, "mysub", ".git"))
	if err != nil || subGitStat.IsDir() {
		t.Fatalf("Submodule .git file was corrupted or deleted by rsync!")
	}
	// Verify git still understands the submodule
	subStatus := runGit(t, filepath.Join(workDir, "mysub"), "status", "--porcelain")
	if subStatus != "" {
		t.Fatalf("Submodule state was altered during Time Travel!")
	}

	// E. Staging Area (Option 2) Flawless Reconstruction
	stagedFiles := runGit(t, workDir, "diff", "--cached", "--name-only")
	if !strings.Contains(stagedFiles, "tracked.txt") {
		t.Fatalf("tracked.txt was not perfectly placed back into the staging area")
	}
	if strings.Contains(stagedFiles, "uncommitted.txt") {
		t.Fatalf("uncommitted.txt was incorrectly staged; it should be untracked")
	}

	// F. Unstaged Modifications (Option 2) Preserved
	unstagedFiles := runGit(t, workDir, "diff", "--name-only")
	if !strings.Contains(unstagedFiles, "tracked.txt") {
		t.Fatalf("tracked.txt partial unstaged modifications were lost during index rebuild")
	}

	// Verify exact content of tracked.txt to prove hunk splitting worked
	trackedData, _ := os.ReadFile(filepath.Join(workDir, "tracked.txt"))
	if string(trackedData) != "v1\npartially-staged-line\nunstaged-line\n" {
		t.Fatalf("tracked.txt content was corrupted during patch application. Got: %s", string(trackedData))
	}

	// 10. Return to Head Check
	runGit(t, workDir, "reset", "--hard", "HEAD")
	runGit(t, workDir, "clean", "-fd")
	runGit(t, workDir, "checkout", "feature-branch")

	futureData, _ := os.ReadFile(filepath.Join(workDir, "future.txt"))
	if string(futureData) != "This is from the future" {
		t.Fatalf("Failed to safely return to the present branch. Future data lost.")
	}
}
