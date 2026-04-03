package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// SnapshotGit executes an add, commit, and tag operation safely.
func SnapshotGit(gitDir, workTree, message, timestamp string) error {
	// 1. git add -A
	cmdAdd := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "add", "-A")
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %v\nGit says: %s", err, strings.TrimSpace(string(out)))
	}

	// 2. git commit (allow empty so we can safely advance the tag even if no files changed)
	cmdCommit := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "commit", "--allow-empty", "-m", message)
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v\nGit says: %s", err, strings.TrimSpace(string(out)))
	}

	// 3. git tag
	cmdTag := exec.Command("git", "--git-dir="+gitDir, "--work-tree="+workTree, "tag", timestamp)
	if out, err := cmdTag.CombinedOutput(); err != nil {
		return fmt.Errorf("git tag failed: %v\nGit says: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}
