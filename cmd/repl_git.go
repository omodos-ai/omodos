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
	count := "15"
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
	}
	fmt.Println()
}

func handleGD(parts []string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	gitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-1", "--pretty=format:%C(cyan)Date:%Creset %ad %C(magenta)(%ar)%Creset\n%C(cyan)HEAD:%Creset %C(yellow)%h%Creset\n%C(cyan)Tags:%Creset %C(green)%D%Creset\n%C(cyan)Msg :%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("\n--- Current State ---")
	cmd.Run()
	fmt.Println("\n")
}

func handleGS(parts []string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		fmt.Println("[Error] Central repo not initialized.")
		return
	}

	// 1. Where are we right now?
	branchCmd := exec.Command("git", "--git-dir="+centralGitDir, "branch", "--show-current")
	branchOut, _ := branchCmd.Output()
	currentlyOnMaster := strings.TrimSpace(string(branchOut)) == "master"

	// 2. What does the user want to do?
	isReturningToPresent := len(parts) == 1 || (len(parts) > 1 && parts[1] == "0")
	timelineIdx := "0"
	if !isReturningToPresent {
		timelineIdx = parts[1]
	}

	// STRICT ENFORCEMENT: No Past -> Past travel allowed.
	if !currentlyOnMaster && !isReturningToPresent {
		fmt.Println("[Error] You are currently viewing a past state.")
		fmt.Println("Please run 'gs' to return to the present before traveling to a different time.")
		return
	}

	cwd, _ := os.Getwd()

	// SCENARIO A: Leaving the Present (Traveling to the Past)
	if currentlyOnMaster && !isReturningToPresent {
		targetCommit := fmt.Sprintf("master~%s", timelineIdx)
		tagCmd := exec.Command("git", "--git-dir="+centralGitDir, "describe", "--tags", "--exact-match", targetCommit)
		tagOut, err := tagCmd.Output()
		if err != nil {
			fmt.Printf("[Error] Could not find a timestamp tag for index [%s].\n", timelineIdx)
			return
		}
		targetTag := strings.TrimSpace(string(tagOut))

		// 1. Snapshot real working tree in shadow timeline just in case
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
				workTree := strings.ReplaceAll(strings.TrimSuffix(entry.Name(), ".git"), "_", "/")
				if stat, err := os.Stat(workTree); err == nil && stat.IsDir() {
					SnapshotGit(shadowGit, workTree, safeMsg, safeTimestamp)
				}
			}
		}
		fmt.Printf("[✓] Timeline safety snapshot saved: %s\n", safeTimestamp)

		// 2. Stash real uncommitted code
		fmt.Printf("Backing up uncommitted code to standard Git stash...\n")
		stashCmd := exec.Command("git", "stash", "push", "-m", fmt.Sprintf("ggo backup before jumping to [%s]", timelineIdx))
		stashCmd.Dir = cwd
		if out, err := stashCmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "No local changes") {
			fmt.Printf("[Info] Stash status: %s\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Println("[✓] Working directory stashed safely.")
		}

		fmt.Printf("\nSwitching timeline to: %s\n", targetTag)

		// 3. Checkout old Central LFS
		exec.Command("git", "--git-dir="+centralGitDir, "--work-tree="+centralRepoDir, "checkout", "-f", targetTag).Run()
		fmt.Println("[✓] Central Context (LFS) restored.")

		// 4. Checkout old Shadow Files into working directory
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".git") {
				shadowGit := filepath.Join(shadowReposDir, entry.Name())
				workTree := strings.ReplaceAll(strings.TrimSuffix(entry.Name(), ".git"), "_", "/")
				if stat, err := os.Stat(workTree); err == nil && stat.IsDir() {
					exec.Command("git", "--git-dir="+shadowGit, "--work-tree="+workTree, "checkout", "-f", targetTag).Run()
				}
			}
		}
		fmt.Printf("\n[✓] Successfully reverted timeline to [%s] %s.\n", timelineIdx, targetTag)
		return
	}

	// SCENARIO B: Returning to the Present (from the Past)
	if !currentlyOnMaster && isReturningToPresent {
		fmt.Println("Clearing past shadow files from working directory...")
		// Hard reset the REAL repo to its own HEAD to destroy any shadow files we extracted
		exec.Command("git", "-C", cwd, "reset", "--hard", "HEAD").Run()

		fmt.Printf("\nSwitching timeline to: master\n")

		// 1. Return Central LFS to master
		exec.Command("git", "--git-dir="+centralGitDir, "--work-tree="+centralRepoDir, "checkout", "-f", "master").Run()
		fmt.Println("[✓] Central Context (LFS) restored.")

		// NOTE: We DO NOT checkout shadow repos when returning to master. 
		// The `git reset --hard HEAD` on the real repo just put your code exactly where it belongs.

		fmt.Printf("\n[✓] Successfully returned to the present (master).\n")
		fmt.Println("Restoring original uncommitted working state...")
		
		// 2. Pop the stash we made when leaving the present
		popCmd := exec.Command("git", "stash", "pop")
		popCmd.Dir = cwd
		if out, err := popCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(out), "No stash entries") {
				fmt.Println("[✓] Working directory is clean (nothing to unstash).")
			} else {
				fmt.Printf("[Warning] Could not auto-unstash: %s\n", string(out))
			}
		} else {
			fmt.Println("[✓] Working directory unstashed successfully.")
		}
		return
	}

	// SCENARIO C: Already on Present, asked to return to Present.
	if currentlyOnMaster && isReturningToPresent {
		fmt.Println("[Info] You are already at the present state (master).")
	}
}

func CleanupTimeMachine() {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("git", "--git-dir="+centralGitDir, "branch", "--show-current")
	out, err := cmd.Output()

	if err == nil && strings.TrimSpace(string(out)) != "master" {
		fmt.Println("\n[ggo] Exiting while time machine is active. Auto-returning to present...")
		handleGS([]string{"gs"})
	}
}
