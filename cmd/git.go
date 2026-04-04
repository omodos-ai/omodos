package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// SyncDirectory uses rsync to forcefully mirror src into dst, ignoring git and ggo internals.
func SyncDirectory(src, dst string) error {
	os.MkdirAll(dst, 0755)
	
	// Ensure trailing slashes for rsync content-copy behavior
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
		
		// Initialize bare shadow if missing
		if _, err := os.Stat(shadowGitDir); os.IsNotExist(err) {
			exec.Command("git", "-C", shadowDir, "init").Run()
		}

		// Blunt Rsync: Working Dir -> Shadow Dir
		if err := SyncDirectory(absDir, shadowDir); err != nil {
			fmt.Printf("[Warning] Failed to sync shadow for %s: %v\n", dir, err)
			continue
		}

		// Commit physical shadow
		if err := SnapshotGit(shadowGitDir, shadowDir, msg, tag); err != nil {
			fmt.Printf("[Warning] Shadow snapshot failed for %s: %v\n", dir, err)
		}
	}
}
