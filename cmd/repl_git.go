package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isTimeTraveling checks if the active_branch file exists, meaning the working dir is in the past.
func isTimeTraveling() bool {
	home, _ := os.UserHomeDir()
	branchFile := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "active_branch")
	_, err := os.Stat(branchFile)
	return err == nil
}

func handleGL(parts []string) {
	count := "15"
	if len(parts) > 1 { count = parts[1] }

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

	if _, err := os.Stat(gitDir); os.IsNotExist(err) { return }

	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-1", "--pretty=format:%C(cyan)Date:%Creset %ad %C(magenta)(%ar)%Creset\n%C(cyan)HEAD:%Creset %C(yellow)%h%Creset\n%C(cyan)Tags:%Creset %C(green)%D%Creset\n%C(cyan)Msg :%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("\n--- Current State ---")
	_ = cmd.Run()
	fmt.Println()
}

// returnToHead replaces the old 'gh' command. Aborts time travel, returning working repo to its original state.
func returnToHead() {
	if !isTimeTraveling() {
		fmt.Println("[Info] You are already at the Head state.")
		return
	}

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	cwd, _ := os.Getwd()

	fmt.Println("Returning working directory to Head...")

	// 1. WIPE Rsync'd shadow files from real working dir
	exec.Command("git", "-C", cwd, "reset", "--hard", "HEAD").Run()
	exec.Command("git", "-C", cwd, "clean", "-fd").Run()

	// 2. Read saved original branch for working directory
	branchFile := filepath.Join(centralRepoDir, "active_branch")
	branchBytes, _ := os.ReadFile(branchFile)
	targetBranch := strings.TrimSpace(string(branchBytes))
	if targetBranch == "" { targetBranch = "master" }

	exec.Command("git", "-C", cwd, "checkout", targetBranch).Run()

	// 3. Unpack the "ggo-temp-rollback" commit, leaving edits safely on the desk
	if out, err := exec.Command("git", "-C", cwd, "reset", "HEAD~1").CombinedOutput(); err != nil {
		fmt.Printf("[Warning] Failed to unpack temp commit: %s\n", out)
	} else {
		fmt.Println("[✓] Temporary rollback commit unpacked. Working state restored.")
	}

	os.Remove(branchFile) // Cleanup tracking file
}

func handleGS(parts []string) {
	// If no arguments are provided, act as the "Return to Present" command.
	if len(parts) == 1 {
		returnToHead()
		return
	}

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		fmt.Println("[Error] Central repo not initialized.")
		return
	}

	if isTimeTraveling() {
		fmt.Println("[Error] You are currently viewing a past state.")
		fmt.Println("Please run 'gs' without arguments to return to Head before traveling to a different time.")
		return
	}

	timelineIdx := parts[1]

	targetCommit := fmt.Sprintf("master~%s", timelineIdx)
	tagCmd := exec.Command("git", "--git-dir="+centralGitDir, "describe", "--tags", "--exact-match", targetCommit)
	tagOut, err := tagCmd.Output()
	if err != nil {
		fmt.Printf("[Error] Could not find a timestamp tag for index [%s].\n", timelineIdx)
		return
	}
	targetTag := strings.TrimSpace(string(tagOut))

	cwd, _ := os.Getwd()

	// 1. Save current working directory branch name
	workBranchCmd := exec.Command("git", "-C", cwd, "branch", "--show-current")
	if wOut, err := workBranchCmd.Output(); err == nil {
		os.WriteFile(filepath.Join(centralRepoDir, "active_branch"), wOut, 0644)
	}

	// 2. Commit uncommitted changes to a temporary safe commit in REAL repo
	fmt.Println("Securing uncommitted working changes into a temporary commit...")
	exec.Command("git", "-C", cwd, "add", "-A").Run()
	exec.Command("git", "-C", cwd, "commit", "-m", "ggo-temp-rollback").Run()

	fmt.Printf("\nExtracting past state (%s) into working directory...\n", targetTag)

	// 3. Checkout Shadow Repos AND Rsync to Working Directory
	for _, dir := range Project.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		safeName := strings.ReplaceAll(strings.TrimPrefix(absDir, "/"), "/", "_")
		shadowDir := filepath.Join(centralRepoDir, "shadows", safeName)
		
		if stat, err := os.Stat(shadowDir); err == nil && stat.IsDir() {
			// Checkout past state in shadow
			exec.Command("git", "-C", shadowDir, "checkout", "-f", targetTag).Run()
			
			// Blunt Rsync: Shadow Dir -> Working Dir
			if err := SyncDirectory(shadowDir, absDir); err != nil {
				fmt.Printf("[Error] Failed to rsync shadow to working dir: %v\n", err)
			}
			
			// Immediately return the physical shadow repo to master so it's ready for the next ingest
			exec.Command("git", "-C", shadowDir, "checkout", "-f", "master").Run()
		}
	}
	
	fmt.Printf("\n[✓] Successfully extracted physical code for [%s] %s.\n", timelineIdx, targetTag)
	fmt.Println("Note: AI Chat Context remains at HEAD so it remembers the future.")
	fmt.Println("Use 'gs' to return code to Head and unpack your working directory.")
}

func CleanupTimeMachine() {
	if isTimeTraveling() {
		fmt.Println("\n[ggo] Exiting while time machine is active. Auto-returning to Head...")
		returnToHead()
	}
}
