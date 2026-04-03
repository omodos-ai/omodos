package cmd

import (
	"fmt"
	"os/exec"
)

// SnapshotGit executes an add, commit, and tag operation safely.
func SnapshotGit(gitDir, workTree, message, timestamp string) error {
	// 1. git add -A
	cmdAdd := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "add", "-A")
	if err := cmdAdd.Run(); err != nil {
		return fmt.Errorf("git add failed: %v", err)
	}

	// 2. git commit (allow empty so we can safely advance the tag even if no files changed)
	cmdCommit := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "commit", "--allow-empty", "-m", message)
	if err := cmdCommit.Run(); err != nil {
		return fmt.Errorf("git commit failed: %v", err)
	}

	// 3. git tag
	cmdTag := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "tag", timestamp)
	if err := cmdTag.Run(); err != nil {
		return fmt.Errorf("git tag failed: %v", err)
	}

	return nil
}
