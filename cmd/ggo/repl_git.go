package ggo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
)

// findGitRoot walks up the directory tree until it finds a .git file or directory.
func findGitRoot(startPath string) string {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // Reached filesystem root
			return ""
		}
		dir = parent
	}
}

func handleGitAddRepo(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: git-add-repo (or gr) <path_to_repo>")
		return
	}

	targetPath := parts[1]
	gitRoot := findGitRoot(targetPath)
	if gitRoot == "" {
		fmt.Printf("[Error] No Git repository found at or above %s\n", targetPath)
		return
	}

	// Check if already tracked
	for _, c := range core.ProjectCfg.CodeDirs {
		if c == gitRoot {
			fmt.Printf("[Info] Repository %s is already tracked by the Time Machine.\n", gitRoot)
			return
		}
	}

	// Enroll it into the Time Machine code_dirs
	core.ProjectCfg.CodeDirs = append(core.ProjectCfg.CodeDirs, gitRoot)
	core.ProjectViper.Set("code_dirs", core.ProjectCfg.CodeDirs)
	core.ProjectViper.WriteConfigAs(filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "project.yaml"))

	fmt.Printf("\033[32m[✓] Enrolled Git repository into Time Machine:\033[0m %s\n", gitRoot)

	// Capture the baseline instantly using the unified transaction engine
	core.ExecuteTransaction("enroll_repo", func(anchor string, now time.Time) error {
		return nil
	})
}

func getLedgerTags() []string {
	centralGitDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), ".git")
	out, err := exec.Command("git", "--git-dir="+centralGitDir, "tag", "--sort=creatordate").CombinedOutput()
	var tags []string
	if err == nil {
		for _, t := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	return tags
}

func handleGL(parts []string) {
	tags := getLedgerTags()
	if len(tags) == 0 {
		fmt.Println("\n[Info] No Time Machine transactions recorded yet.")
		return
	}
	fmt.Println("\n--- Time Machine Ledger ---")
	centralGitDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), ".git")
	for i, t := range tags {
		// Extract the human-readable commit message stored in the central vault
		out, err := exec.Command("git", "--git-dir="+centralGitDir, "log", "-1", "--format=%s", t).Output()
		msg := ""
		if err == nil {
			msg = strings.TrimSpace(string(out))
		}
		fmt.Printf("[%d] %-25s %s\n", i, t, msg)
	}
	fmt.Println()
}

func handleGS(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: gs <index>")
		return
	}
	idx, err := strconv.Atoi(parts[1])
	tags := getLedgerTags()
	if err != nil || idx < 0 || idx >= len(tags) {
		fmt.Println("[Error] Invalid ledger index. Run 'gl' to view valid indices.")
		return
	}

	tag := tags[idx]
	lastDash := strings.LastIndex(tag, "-")
	if lastDash == -1 {
		fmt.Println("[Error] Malformed transaction tag:", tag)
		return
	}
	anchor := tag[:lastDash]
	phase := tag[lastDash+1:]

	fmt.Printf("\033[35m[ggo] Traveling through time to %s...\033[0m\n", tag)

	for _, dir := range core.ProjectCfg.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		repoID := core.GetRepoID(absDir)
		shadowDir := filepath.Join(core.GetConfigDir(), "shadow-repos", repoID)

		err := core.Gitsync(core.SyncParams{
			Direction: "ShadowToWorking",
			WorkDir:   absDir,
			ShadowDir: shadowDir,
			Anchor:    anchor,
			Phase:     phase,
		})
		if err != nil {
			fmt.Printf("\033[31m[Warning] Failed to restore %s: %v\033[0m\n", dir, err)
		} else {
			fmt.Printf("[✓] Restored workspace: %s\n", dir)
		}
	}
}

func handleGD(parts []string) {
	if len(parts) == 2 {
		// Check if the user passed a ledger index (e.g., `gd 5`)
		if idx, err := strconv.Atoi(parts[1]); err == nil {
			tags := getLedgerTags()
			if idx >= 0 && idx < len(tags) {
				tag := tags[idx]
				fmt.Printf("\n--- Time Machine Diff for Index [%d] %s ---\n", idx, tag)

				for _, dir := range core.ProjectCfg.CodeDirs {
					absDir, _ := filepath.Abs(dir)
					repoID := core.GetRepoID(absDir)
					shadowDir := filepath.Join(core.GetConfigDir(), "shadow-repos", repoID)
					refPath := "refs/ggo/txn/" + tag

					// Ask the shadow repo to show the diff for this specific transaction
					cmd := exec.Command("git", "-C", shadowDir, "show", "--stat", "-p", refPath)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()
				}
				return
			}
		}
	}

	// Fallback: If no index is provided, behave as a standard pass-through to `git diff`
	cwd, _ := os.Getwd()
	args := append([]string{"diff"}, parts[1:]...)
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Ignore exit status 1, which git natively uses to indicate diffs exist
	}
}
