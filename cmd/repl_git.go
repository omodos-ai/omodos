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

	// 1. Anchor Time Travel to the absolute present ("master"), not the relative "HEAD"
	targetCommit := "master"
	timelineIdx := "0"
	if len(parts) > 1 {
		targetCommit = fmt.Sprintf("master~%s", parts[1])
		timelineIdx = parts[1]
	}

	isReturningToPresent := (timelineIdx == "0")

	// 2. Resolve the exact tag for that commit
	tagCmd := exec.Command("git", "--git-dir="+centralGitDir, "describe", "--tags", "--exact-match", targetCommit)
	tagOut, err := tagCmd.Output()
	if err != nil {
		fmt.Printf("[Error] Could not find a timestamp tag for index [%s].\n", timelineIdx)
		return
	}
	targetTag := strings.TrimSpace(string(tagOut))

	// 3. Safety First: Create a Timeline Snapshot BEFORE rewinding (skip if returning to present)
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

	// 4. Stash uncommitted user changes
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

	fmt.Printf("\nSwitching timeline to: [%s] %s\n", timelineIdx, targetTag)

	// 5. Restore the Central Repo (AI Brain)
	// Notice we checkout targetTag, keeping HEAD detached in the past cleanly!
	checkoutCentral := exec.Command("git", "--git-dir="+centralGitDir, "--work-tree="+centralRepoDir, "checkout", targetTag)
	if err := checkoutCentral.Run(); err != nil {
		fmt.Printf("[Error] Failed to restore central repo: %v\n", err)
		return
	}
	fmt.Println("[✓] Central Context (LFS) restored.")

	// 6. Restore all associated Shadow Repos (Code Hands)
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
					// Ignore safely
				} else {
					fmt.Printf("[✓] Code state restored: %s\n", workTree)
				}
			}
		}
	}

	fmt.Printf("\n[✓] Successfully reverted timeline to [%s] %s.\n", timelineIdx, targetTag)

	// 7. Auto-Cleanup: Pop the stash if we are returning to the present
	if isReturningToPresent {
		fmt.Println("\nReturning to present: Restoring uncommitted working state...")
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
