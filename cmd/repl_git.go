package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func isTimeTraveling() bool {
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	matches, _ := filepath.Glob(filepath.Join(centralRepoDir, "branch_*"))
	return len(matches) > 0
}

func handleGL(parts []string) {
	count := "15"
	if len(parts) > 1 {
		count = parts[1]
	}

	if len(Project.CodeDirs) == 0 {
		fmt.Println("[Error] No repositories enrolled. Run 'inc <path>' to start tracking.")
		return
	}

	home, _ := os.UserHomeDir()
	absDir, _ := filepath.Abs(Project.CodeDirs[0])
	repoID := getRepoID(absDir)
	
	shadowDir := filepath.Join(home, ".ggo", "shadow-repos", repoID)

	cmd := exec.Command("git", "-C", shadowDir, "for-each-ref", "--sort=-creatordate", "--format=%(refname:lstrip=3)|%(contents:subject)", "refs/ggo/txn/")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("[Error] Timeline not initialized or shadow vault missing.")
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	fmt.Printf("\n--- AI Transaction Timeline (Max %s) ---\n", count)

	seen := make(map[string]bool)
	idx := 0
	limit := 15
	fmt.Sscanf(count, "%d", &limit)

	metaRegex := regexp.MustCompile(`\[branch:\s*[^\]]+\]\s*\[base:\s*[^\]]+\]\s*`)

	for _, line := range lines {
		if line == "" || idx >= limit {
			continue
		}
		data := strings.SplitN(line, "|", 2)
		if len(data) < 2 {
			continue
		}

		refName := data[0]
		subject := data[1]

		anchorParts := strings.Split(refName, "-")
		if len(anchorParts) < 2 {
			continue
		}
		anchor := anchorParts[0]

		if seen[anchor] {
			continue
		}
		seen[anchor] = true

		subject = metaRegex.ReplaceAllString(subject, "")
		subject = strings.Replace(subject, "after: ", "", 1)
		subject = strings.Replace(subject, "before: ", "(aborted) ", 1)
		subject = strings.Replace(subject, "transaction started: ", "(failed) ", 1)
		subject = strings.Replace(subject, "init: ", "(baseline) ", 1)

		fmt.Printf("[\033[32m%d\033[0m] \033[36m%s\033[0m | %s\n", idx, anchor, subject)
		idx++
	}
	fmt.Println()
}

func handleGD(parts []string) {
	cwd, _ := os.Getwd()
	cmd := exec.Command("git", "-C", cwd, "log", "-1", "--pretty=format:%C(cyan)Date:%Creset %ad %C(magenta)(%ar)%Creset\n%C(cyan)HEAD:%Creset %C(yellow)%h%Creset\n%C(cyan)Tags:%Creset %C(green)%D%Creset\n%C(cyan)Msg :%Creset %s", "--date=format:%Y-%m-%d %H:%M:%S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("\n--- Current State ---")
	_ = cmd.Run()
	fmt.Println()
}

func returnToHead() {
	if !isTimeTraveling() {
		fmt.Println("[Info] You are already in the present.")
		return
	}

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)

	fmt.Printf("Returning working directories to present...\n")

	for _, dir := range Project.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		repoID := getRepoID(absDir)
		branchFile := filepath.Join(centralRepoDir, "branch_"+repoID)

		branchBytes, err := os.ReadFile(branchFile)
		if err == nil {
			targetBranch := strings.TrimSpace(string(branchBytes))
			exec.Command("git", "-C", absDir, "reset", "--hard", "HEAD").Run()
			exec.Command("git", "-C", absDir, "clean", "-fd").Run()
			exec.Command("git", "-C", absDir, "checkout", targetBranch).Run()
			os.Remove(branchFile)
		}
	}

	fmt.Println("[✓] Timeline restored. You are back in the present.")
}

func handleGS(parts []string) {
	if len(parts) == 1 {
		returnToHead()
		return
	}

	if isTimeTraveling() {
		fmt.Println("[Error] You are currently viewing a past state.")
		fmt.Println("Please run 'gs' without arguments to return to the present before traveling again.")
		return
	}

	anchor := parts[1]
	phase := "after"
	if len(parts) > 2 {
		phase = parts[2]
	}

	if len(anchor) < 5 {
		anchor = resolveIndexToAnchor(anchor)
		if anchor == "" {
			fmt.Println("[Error] Invalid timeline index.")
			return
		}
	}

	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)

	fmt.Printf("\nExtracting past state (%s-%s) into working directories...\n", anchor, phase)

	for _, dir := range Project.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		repoID := getRepoID(absDir)
		
		branchFile := filepath.Join(centralRepoDir, "branch_"+repoID)
		workBranchCmd := exec.Command("git", "-C", absDir, "branch", "--show-current")
		if wOut, err := workBranchCmd.Output(); err == nil && string(wOut) != "" {
			os.WriteFile(branchFile, []byte(strings.TrimSpace(string(wOut))), 0644)
		}

		shadowDir := filepath.Join(home, ".ggo", "shadow-repos", repoID)

		err := Gitsync(SyncParams{
			Direction: "ShadowToWorking",
			WorkDir:   absDir,
			ShadowDir: shadowDir,
			Anchor:    anchor,
			Phase:     phase,
		})
		if err != nil {
			fmt.Printf("[Error] Failed to restore %s: %v\n", dir, err)
		}
	}

	fmt.Printf("\n[✓] Successfully vaulted to %s-%s.\n", anchor, phase)
	fmt.Println("Note: You are in a Detached HEAD state to protect your future commits.")
	fmt.Println("Use 'gs' to return to the present.")
}

func resolveIndexToAnchor(idxStr string) string {
	if len(Project.CodeDirs) == 0 {
		fmt.Println("[Error] No repositories tracked.")
		return ""
	}
	home, _ := os.UserHomeDir()
	absDir, _ := filepath.Abs(Project.CodeDirs[0])
	repoID := getRepoID(absDir)
	
	shadowDir := filepath.Join(home, ".ggo", "shadow-repos", repoID)

	cmd := exec.Command("git", "-C", shadowDir, "for-each-ref", "--sort=-creatordate", "--format=%(refname:lstrip=3)", "refs/ggo/txn/")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var anchors []string
	seen := make(map[string]bool)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "-")
		if len(parts) < 2 {
			continue
		}
		anchor := parts[0]
		if !seen[anchor] {
			seen[anchor] = true
			anchors = append(anchors, anchor)
		}
	}

	targetIdx := 0
	fmt.Sscanf(idxStr, "%d", &targetIdx)
	if targetIdx >= 0 && targetIdx < len(anchors) {
		return anchors[targetIdx]
	}
	return ""
}

func CleanupTimeMachine() {
	if isTimeTraveling() {
		fmt.Println("\n[ggo] Exiting while time machine is active. Auto-returning to Head...")
		returnToHead()
	}
}
