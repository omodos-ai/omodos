package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// Shows the commit hash, date, tags, and the commit message
	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-n", count, "--pretty=format:%C(yellow)%h%Creset %C(cyan)%ad%Creset | %C(green)%D%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	fmt.Printf("\n--- Project Timeline (Last %s) ---\n", count)
	err := cmd.Run()
	fmt.Println("\n")
	
	if err != nil {
		fmt.Printf("[Error] Failed to list history: %v\n", err)
	}
}

func handleGD(parts []string) {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	gitDir := filepath.Join(centralRepoDir, ".git")

	cmd := exec.Command("git", "--git-dir="+gitDir, "log", "-1", "--pretty=format:%C(cyan)Date:%Creset %ad\n%C(cyan)HEAD:%Creset %C(yellow)%h%Creset\n%C(cyan)Tags:%Creset %C(green)%D%Creset\n%C(cyan)Msg :%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	fmt.Println("\n--- Current State ---")
	err := cmd.Run()
	fmt.Println("\n")
	
	if err != nil {
		fmt.Printf("[Error] Failed to describe state: %v\n", err)
	}
}
