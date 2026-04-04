package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecuteTransaction wraps any state-altering function in a deterministic before/after commit bracket.
func ExecuteTransaction(slug string, action func(timestamp string, now time.Time) error) error {
	now := time.Now().UTC()
	baseTimestamp := now.Format("20060102T150405Z")
	beforeTag := baseTimestamp + "-before"
	afterTag := baseTimestamp + "-after"

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	// 1. Intercept: Project Initialization is an explicit transaction
	isInit := false
	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		isInit = true
		os.MkdirAll(centralRepoDir, 0755)
		exec.Command("git", "-C", centralRepoDir, "init").Run()
	}

	// 2. Always mirror the latest local config to the central repo so it is tracked
	localYaml := filepath.Join(".ggo", "project.yaml")
	centralYaml := filepath.Join(centralRepoDir, "project.yaml")
	if input, err := os.ReadFile(localYaml); err == nil {
		os.WriteFile(centralYaml, input, 0644)
	}

	if isInit {
		fmt.Printf("\033[90m[ggo] Initializing central repository tracking...\033[0m\n")
		CommitGlobalTransaction("system: project initialized", baseTimestamp+"-init")
	}

	fmt.Printf("\033[90m[ggo] Securing 'before' transaction state (%s)...\033[0m\n", beforeTag)
	CommitGlobalTransaction(fmt.Sprintf("before: %s", slug), beforeTag)

	// Execute the actual command logic
	err := action(baseTimestamp, now)

	fmt.Printf("\033[90m[ggo] Securing 'after' transaction state (%s)...\033[0m\n", afterTag)
	CommitGlobalTransaction(fmt.Sprintf("after: %s", slug), afterTag)

	return err
}

// SnapshotGit executes an add, commit, and tag operation safely.
func SnapshotGit(gitDir, workTree, message, timestamp string) error {
	cmdAdd := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "add", "-A")
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %v\nOutput: %s", err, string(out))
	}

	cmdCommit := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "commit", "--allow-empty", "-m", message)
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v\nOutput: %s", err, string(out))
	}

	cmdTag := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "tag", timestamp)
	if out, err := cmdTag.CombinedOutput(); err != nil {
		return fmt.Errorf("git tag failed: %v\nOutput: %s", err, string(out))
	}

	return nil
}

// SyncDirectory uses rsync to forcefully mirror src into dst.
func SyncDirectory(src, dst string) error {
	os.MkdirAll(dst, 0755)
	src = strings.TrimSuffix(src, "/") + "/"
	dst = strings.TrimSuffix(dst, "/") + "/"

	cmd := exec.Command("rsync", "-a", "--delete", "--exclude=.git/", "--exclude=.ggo/", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsync failed: %s", string(out))
	}
	return nil
}

// CommitGlobalTransaction locks the state of the LFS repo and all shadow repos simultaneously.
func CommitGlobalTransaction(msg, tag string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	// 1. Snapshot Central LFS
	if err := SnapshotGit(centralGitDir, centralRepoDir, msg, tag); err != nil {
		fmt.Printf("[Warning] Central snapshot failed: %v\n", err)
	}

	// 2. Snapshot Physical Shadow Repos
	for _, dir := range Project.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		safeName := strings.ReplaceAll(strings.TrimPrefix(absDir, "/"), "/", "_")
		shadowDir := filepath.Join(centralRepoDir, "shadows", safeName)
		
		os.MkdirAll(shadowDir, 0755)
		shadowGitDir := filepath.Join(shadowDir, ".git")
		
		if _, err := os.Stat(shadowGitDir); os.IsNotExist(err) {
			exec.Command("git", "-C", shadowDir, "init").Run()
		}

		if err := SyncDirectory(absDir, shadowDir); err != nil {
			fmt.Printf("[Warning] Failed to sync shadow for %s: %v\n", dir, err)
			continue
		}

		if err := SnapshotGit(shadowGitDir, shadowDir, msg, tag); err != nil {
			fmt.Printf("[Warning] Shadow snapshot failed for %s: %v\n", dir, err)
		}
	}
}
