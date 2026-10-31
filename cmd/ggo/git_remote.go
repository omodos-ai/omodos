// Copyright 2025-2026 Peter Braam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ggo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/viper"
)

func handleGitRemote(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: gr <push|pull|clone> [project_name]")
		return
	}

	action := parts[1]
	projectName := core.ProjectCfg.ProjectName
	if len(parts) >= 3 {
		projectName = parts[2]
	}

	if projectName == "" {
		fmt.Println("[Error] No active project and no project name provided.")
		return
	}

	vaultDir := filepath.Join(core.GetConfigDir(), "ggoai-"+projectName)
	vaultRepoName := "ggoai-" + projectName

	switch action {
	case "push":
		fmt.Println("\n--- Pushing Central Vault ---")
		pushRepo(vaultDir, vaultRepoName)

		if projectName == core.ProjectCfg.ProjectName {
			fmt.Println("\n--- Pushing Enrolled Code Repositories ---")
			for _, codeDir := range core.ProjectCfg.CodeDirs {
				dirName := filepath.Base(codeDir)
				pushRepo(codeDir, dirName)
			}
		}

	case "pull":
		fmt.Println("\n--- Pulling Central Vault ---")
		pullRepo(vaultDir)

		if projectName == core.ProjectCfg.ProjectName {
			fmt.Println("\n--- Pulling Enrolled Code Repositories ---")
			for _, codeDir := range core.ProjectCfg.CodeDirs {
				pullRepo(codeDir)
			}
		}

	case "clone":
		fmt.Printf("\n--- Cloning Project: %s ---\n", projectName)
		cloneRepo(vaultRepoName, vaultDir)

		// Parse the newly cloned vault to auto-clone code repos
		yamlPath := filepath.Join(vaultDir, "project.yaml")
		v := viper.New()
		v.SetConfigFile(yamlPath)
		v.SetConfigType("yaml")

		if err := v.ReadInConfig(); err == nil {
			codeDirs := v.GetStringSlice("code_dirs")
			if len(codeDirs) > 0 {
				fmt.Println("\n--- Auto-Cloning Enrolled Code Repositories ---")
				cwd, _ := os.Getwd()
				for _, cd := range codeDirs {
					base := filepath.Base(cd)
					target := filepath.Join(cwd, base)
					cloneRepo(base, target)
				}
			}
		}
		fmt.Printf("\n[✓] Clone complete! You can now run 'ggo --new' to activate the workspace.\n")
	}
}

func pushRepo(dir, defaultName string) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		fmt.Printf("[Skip] %s is not a git repository.\n", dir)
		return
	}

	fmt.Printf("Syncing %s...\n", defaultName)

	// Auto-commit any outstanding changes before pushing
	exec.Command("git", "-C", dir, "add", "-A").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "ggo auto-sync").Run()

	// Check if a remote exists
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	if err := cmd.Run(); err != nil {
		fmt.Printf("[Info] No remote 'origin' found. Creating private GitHub repository '%s'...\n", defaultName)

		// Note: Requires GitHub CLI (gh) installed and authenticated
		createCmd := exec.Command("gh", "repo", "create", defaultName, "--private", "--source=.", "--remote=origin", "--push")
		createCmd.Dir = dir
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			fmt.Printf("\n\033[31m[Error]\033[0m Failed to create GitHub repo. Ensure GitHub CLI is installed and you ran 'gh auth login'.\n")
		} else {
			fmt.Printf("[✓] Created and pushed %s to GitHub.\n", defaultName)
		}
	} else {
		// Remote exists, just push
		pushCmd := exec.Command("git", "-C", dir, "push", "origin", "HEAD")
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			fmt.Printf("\033[31m[Error]\033[0m Failed to push: %v\n", err)
		} else {
			fmt.Printf("[✓] Successfully pushed %s.\n", defaultName)
		}
	}
}

func pullRepo(dir string) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		fmt.Printf("[Skip] %s is not a git repository.\n", dir)
		return
	}

	fmt.Printf("Pulling %s...\n", filepath.Base(dir))
	pullCmd := exec.Command("git", "-C", dir, "pull", "--rebase")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		fmt.Printf("\033[31m[Error]\033[0m Failed to pull: %v\n", err)
	} else {
		fmt.Printf("[✓] Pulled latest changes.\n")
	}
}

func cloneRepo(repoName, destDir string) {
	fmt.Printf("Fetching %s...\n", repoName)
	cloneCmd := exec.Command("gh", "repo", "clone", repoName, destDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		fmt.Printf("\033[31m[Error]\033[0m Failed to clone GitHub repo '%s'. Does it exist?\n", repoName)
	} else {
		fmt.Printf("[✓] Successfully cloned to %s.\n", destDir)
	}
}
