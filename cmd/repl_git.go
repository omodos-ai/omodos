package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func handleGL(parts []string) {
	count := "15" // Default to last 15 actions
	if len(parts) > 1 {
		count = parts[1]
	}

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	gitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Println("[Error] Central repo not initialized.")
		return
	}

	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-n", count, "--color=always", "--pretty=format:%C(yellow)%h%Creset %C(cyan)%ad %C(magenta)(%ar)%Creset | %C(green)%D%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("[Error] Failed to list history: %v\n", err)
		return
	}

	fmt.Printf("\n--- Project Timeline (Last %s) ---\n", count)

	if len(out) > 0 {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for i, line := range lines {
			fmt.Printf("[\033[32m%d\033[0m] %s\n", i, line)
		}
	} else {
		fmt.Println("No history found.")
	}
	fmt.Println()
}

func handleGD(parts []string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	gitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Println("[Error] Central repo not initialized.")
		return
	}

	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-1", "--pretty=format:%C(cyan)Date:%Creset %ad %C(magenta)(%ar)%Creset\n%C(cyan)HEAD:%Creset %C(yellow)%h%Creset\n%C(cyan)Tags:%Creset %C(green)%D%Creset\n%C(cyan)Msg :%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("\n--- Current State ---")
	err := cmd.Run()
	fmt.Println("\n")

	if err != nil {
		fmt.Printf("[Error] Failed to describe state: %v\n", err)
	}
}

func handleGS(parts []string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		fmt.Println("[Error] Central repo not initialized.")
		return
	}

	// 1. Determine Intent
	isReturningToPresent := len(parts) == 1
	timelineIdx := "0"
	if !isReturningToPresent {
		timelineIdx = parts[1]
	}

	// 2. Resolve Tag
	var targetTag string
	if isReturningToPresent {
		targetTag = "master"
	} else {
		targetCommit := fmt.Sprintf("HEAD~%s", timelineIdx)
		tagCmd := exec.Command("git", "--git-dir="+centralGitDir, "describe", "--tags", "--exact-match", targetCommit)
		tagOut, err := tagCmd.Output()
		if err != nil {
			fmt.Printf("[Error] Could not find a timestamp tag for index [%s]. Ensure it has a valid tag.\n", timelineIdx)
			return
		}
		targetTag = strings.TrimSpace(string(tagOut))
	}

	// 3. Safety Snapshot
	if !isReturningToPresent {
		fmt.Println("Creating safety snapshot in shadow timeline...")
		now := time.Now()
		safeTimestamp := now.Format("20060102T150405")
		safeMsg := fmt.Sprintf("auto-backup before rewinding to [%s]", timelineIdx)

		SnapshotGit(centralGitDir, centralRepoDir, safeMsg, safeTimestamp)

		shadowReposDir := filepath.Join(centralRepoDir, "shadow-repos")
		entries, _ := os.ReadDir(shadowReposDir)
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".git") {
				shadowGit := filepath.Join(shadowReposDir, entry.Name())
				safeName := strings.TrimSuffix(entry.Name(), ".git")
				workTree := strings.ReplaceAll(safeName, "_", "/")

				if stat, err := os.Stat(workTree); err == nil && stat.IsDir() {
					SnapshotGit(shadowGit, workTree, safeMsg, safeTimestamp)
				}
			}
		}
		fmt.Printf("[✓] Timeline safety snapshot saved: %s\n", safeTimestamp)
	}

	// 4. Stash uncommitted changes
	cwd, _ := os.Getwd()
	fmt.Printf("Backing up uncommitted code to standard Git stash...\n")
	stashCmd := exec.Command("git", "stash", "push", "-m", fmt.Sprintf("ggo backup before jumping to [%s]", timelineIdx))
	stashCmd.Dir = cwd
	if out, err := stashCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "No local changes") {
			fmt.Printf("[Info] Stash status: %s\n", strings.TrimSpace(string(out)))
		}
	} else {
		fmt.Println("[✓] Working directory stashed safely.")
	}

	fmt.Printf("\nSwitching timeline to: %s\n", targetTag)

	// 5. Restore Central Repo
	checkoutCentral := exec.Command("git", "--git-dir="+centralGitDir, "--work-tree="+centralRepoDir, "checkout", "-f", targetTag)
	if out, err := checkoutCentral.CombinedOutput(); err != nil {
		fmt.Printf("[Error] Failed to restore central repo: %v\nOutput: %s\n", err, string(out))
		return
	}
	fmt.Println("[✓] Central Context (LFS) restored.")

	// 6. Restore Shadow Repos
	shadowReposDir := filepath.Join(centralRepoDir, "shadow-repos")
	entries, _ := os.ReadDir(shadowReposDir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".git") {
			shadowGit := filepath.Join(shadowReposDir, entry.Name())
			safeName := strings.TrimSuffix(entry.Name(), ".git")
			workTree := strings.ReplaceAll(safeName, "_", "/")

			if stat, err := os.Stat(workTree); err == nil && stat.IsDir() {
				checkoutShadow := exec.Command("git", "--git-dir="+shadowGit, "--work-tree="+workTree, "checkout", "-f", targetTag)
				if err := checkoutShadow.Run(); err != nil {
					// Safely ignore
				} else {
					fmt.Printf("[✓] Code state restored: %s\n", workTree)
				}
			}
		}
	}

	if isReturningToPresent {
		fmt.Printf("\n[✓] Successfully returned to the present (master).\n")
	} else {
		fmt.Printf("\n[✓] Successfully reverted timeline to [%s] %s.\n", timelineIdx, targetTag)
	}

	// 7. Auto-Cleanup
	if isReturningToPresent {
		fmt.Println("Restoring uncommitted working state...")
		popCmd := exec.Command("git", "stash", "pop")
		popCmd.Dir = cwd
		if out, err := popCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(out), "No stash entries") {
				fmt.Println("[✓] Working directory is clean (nothing to unstash).")
			} else {
				fmt.Printf("[Warning] Could not auto-unstash (you may have conflicts): %s\n", string(out))
			}
		} else {
			fmt.Println("[✓] Working directory unstashed successfully.")
		}
	}
}

// CleanupTimeMachine checks if we are currently time-traveling (not on master)
// and forces a return to the present to prevent leaving the user in a detached state.
func CleanupTimeMachine() {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("git", "--git-dir="+centralGitDir, "branch", "--show-current")
	out, err := cmd.Output()
	
	// If output is empty, we are in a detached HEAD state (time traveling)
	if err == nil && strings.TrimSpace(string(out)) != "master" {
		fmt.Println("\n[ggo] Exiting while time machine is active. Auto-returning to present...")
		handleGS([]string{"gs"})
	}
}
