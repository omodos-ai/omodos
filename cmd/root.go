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
		
		if Project.ProjectName != "" {
			defaultName = Project.ProjectName
		}
		
		projDir := filepath.Join(home, ".ggo", "ggoai-"+defaultName)
		projectFilePath := filepath.Join(projDir, "project.yaml")

		// 1. Check if the project is fully scaffolded on disk
		needsInit := false
		if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
			needsInit = true
		}

		if needsInit {
			// 2. Prompt the user
			fmt.Printf("[?] This directory is missing its central ggo scaffolding.\n")
			fmt.Printf("Initialize ggo project '%s' here? [Y/n]: ", defaultName)
			
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			
			if response == "" || response == "y" || response == "yes" {
				// Scaffold the directories in the centralized ~/.ggo/ location
				os.MkdirAll(projDir, 0755)
				os.MkdirAll(filepath.Join(projDir, "chats", "source"), 0755)
				os.MkdirAll(filepath.Join(projDir, "includes"), 0755)
				os.MkdirAll(filepath.Join(home, ".ggo", "shadow-repos"), 0755)
				
				defaultYaml := `code_dirs:
  - "."
included_data: []
shell_cmd: "bash"
diff_cmd: "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'"
`
				err := os.WriteFile(projectFilePath, []byte(defaultYaml), 0644)
				if err != nil {
					fmt.Printf("[!] Failed to create project file: %v\n", err)
				} else {
					fmt.Printf("[✓] Created %s\n", projectFilePath)
					
					// Register project in global config using absolute path
					if Global.Projects == nil {
						Global.Projects = make(map[string]string)
					}
					Global.Projects[defaultName] = cwd
					GlobalViper.Set("projects", Global.Projects)
					
					if err := GlobalViper.WriteConfigAs(filepath.Join(home, ".ggo", "config.yaml")); err != nil {
						fmt.Printf("[!] Failed to save global config: %v\n", err)
					}

					// Initialize Git and Git LFS in the central directory
					initGitLFS(projDir)

					// Force Viper to read the newly created config immediately
					InitConfig()
				}
			} else {
				fmt.Println("Skipping project initialization. Running with global defaults.\n")
			}
		}

		// 3. Launch the REPL!
		RunREPL()
	},
}

func initGitLFS(projDir string) {
	fmt.Printf("Initializing Git LFS in %s...\n", projDir)
	
	// git init
	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = projDir
	if err := cmdInit.Run(); err != nil {
		fmt.Printf("[Warning] Failed to run 'git init': %v\n", err)
		return
	}

	// git lfs install
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
