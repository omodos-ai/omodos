package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ggo",
	Short: "A strict, Unix-style REPL for knowledge engineering",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Check if the local project file exists
		projectFilePath := filepath.Join(".ggo", "project.yaml")
		if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
			
			// 2. Prompt the user
			fmt.Printf("[?] No .ggo/project.yaml found in current directory.\n")
			fmt.Printf("Initialize a new ggo project here? [Y/n]: ")
			
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			
			if response == "" || response == "y" || response == "yes" {
				// Scaffold the directory and file
				os.MkdirAll(".ggo", 0755)
				
				// Get the current directory name to use as default project name
				dir, _ := os.Getwd()
				defaultName := filepath.Base(dir)

				defaultYaml := fmt.Sprintf(`project_name: "%s"
code_dirs:
  - "."
included_data: []
shell_cmd: "bash"
diff_cmd: "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'"
`, defaultName)

				err := os.WriteFile(projectFilePath, []byte(defaultYaml), 0644)
				if err != nil {
					fmt.Printf("[!] Failed to create project file: %v\n", err)
				} else {
					fmt.Printf("[✓] Created %s\n\n", projectFilePath)
					// Force Viper to read the newly created config immediately
					InitConfig()
				}
			} else {
				fmt.Println("Skipping project initialization. Running with global defaults.")
				fmt.Println()
			}
		}

		// 3. Launch the REPL!
		RunREPL()
		fmt.Println("Welcome to ggo. (REPL scaffolding coming next...)")
	},
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
