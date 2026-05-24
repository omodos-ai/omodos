package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	for _, c := range Project.CodeDirs {
		if c == gitRoot {
			fmt.Printf("[Info] Repository %s is already tracked by the Time Machine.\n", gitRoot)
			return
		}
	}

	// Enroll it into the Time Machine code_dirs
	Project.CodeDirs = append(Project.CodeDirs, gitRoot)
	ProjectViper.Set("code_dirs", Project.CodeDirs)
	ProjectViper.WriteConfigAs(filepath.Join(GetProjectVaultDir(Project.ProjectName), "project.yaml"))

	fmt.Printf("\033[32m[✓] Enrolled Git repository into Time Machine:\033[0m %s\n", gitRoot)

	// Capture the baseline instantly using the unified transaction engine
	ExecuteTransaction("enroll_repo", func(anchor string, now time.Time) error {
		return nil
	})
}

func getLedgerTags() []string {
	centralGitDir := filepath.Join(GetProjectVaultDir(Project.ProjectName), ".git")
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
	for i, t := range tags {
		fmt.Printf("[%d] %s\n", i, t)
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

	for _, dir := range Project.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		repoID := getRepoID(absDir)
		shadowDir := filepath.Join(GetConfigDir(), "shadow-repos", repoID)

		err := Gitsync(SyncParams{
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
	fmt.Println("-> Stub: Time Machine diff (gd) is pending UI wiring.")
}
