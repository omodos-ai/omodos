package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ggo",
	Short: "A strict, Unix-style REPL for knowledge engineering",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := os.Getwd()
		cwd, _ = filepath.Abs(cwd)
		defaultName := filepath.Base(cwd)
		home, _ := os.UserHomeDir()

		needsInit := (Project.ProjectName == "")

		if needsInit {
			fmt.Printf("\n[?] This directory is not tracked by ggo.\n")
			fmt.Printf("Initialize new ggo project tracking here? [Y/n]: ")

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "" || response == "y" || response == "yes" {

				fmt.Printf("Project name [\033[36m%s\033[0m]: ", defaultName)
				nameResp, _ := reader.ReadString('\n')
				nameResp = strings.TrimSpace(nameResp)
				if nameResp != "" {
					defaultName = nameResp
				}

				projDir := filepath.Join(home, ".ggo", "ggoai-"+defaultName)
				os.MkdirAll(projDir, 0755)
				os.MkdirAll(filepath.Join(projDir, "chats", "source"), 0755)
				os.MkdirAll(filepath.Join(projDir, "includes"), 0755)

				// Strictly initialize with NO code directories. Wait for 'inc'.
				defaultYaml := fmt.Sprintf(`project_name: "%s"
code_dirs: []
included_data: []
shell_cmd: "bash"
diff_cmd: "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'"
model_name: "gemini-3.1-pro-preview"
`, defaultName)

				centralYamlPath := filepath.Join(projDir, "project.yaml")
				err := os.WriteFile(centralYamlPath, []byte(defaultYaml), 0644)
				if err != nil {
					fmt.Printf("[!] Failed to create project file in central vault: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("[✓] Created %s\n", centralYamlPath)

				if Global.Projects == nil {
					Global.Projects = make(map[string]string)
				}
				Global.Projects[defaultName] = cwd
				GlobalViper.Set("projects", Global.Projects)

				if err := GlobalViper.WriteConfigAs(filepath.Join(home, ".ggo", "config.yaml")); err != nil {
					fmt.Printf("[!] Failed to save global config: %v\n", err)
				}

				initGitLFS(projDir)
				InitConfig()

				fmt.Printf("[Info] Project initialized. Waiting for 'inc <path>' to enroll repositories.\n")

			} else {
				fmt.Println("Skipping project initialization. Exiting.")
				os.Exit(0)
			}
		} else {
			projDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
			if _, err := os.Stat(projDir); os.IsNotExist(err) {
				fmt.Printf("[Info] Restoring missing central vault for project '%s'...\n", Project.ProjectName)
				os.MkdirAll(projDir, 0755)
				os.MkdirAll(filepath.Join(projDir, "chats", "source"), 0755)
				os.MkdirAll(filepath.Join(projDir, "includes"), 0755)
				initGitLFS(projDir)
			}
		}

		RunREPL()
	},
}

func initGitLFS(projDir string) {
	fmt.Printf("Initializing Git LFS in %s...\n", projDir)
	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = projDir
	if err := cmdInit.Run(); err != nil {
		fmt.Printf("[Warning] Failed to run 'git init': %v\n", err)
		return
	}
	cmdLFS := exec.Command("git", "lfs", "install")
	cmdLFS.Dir = projDir
	if err := cmdLFS.Run(); err != nil {
		fmt.Printf("[Warning] Failed to run 'git lfs install'. Is Git LFS installed on this machine? (%v)\n", err)
	} else {
		fmt.Printf("[✓] Git LFS initialized.\n")
	}
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(InitConfig)
}
