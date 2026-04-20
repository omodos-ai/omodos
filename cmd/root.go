package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgDirFlag string

var rootCmd = &cobra.Command{
	Use:   "ggo",
	Short: "Omodos AI - Headless Git-aware Unix orchestrator",
	Long:  `Omodos AI (ggo) manages AI interactions for knowledge management, code generation, and co-authoring with strict data sovereignty.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If executed without arguments, default to starting the REPL
		fmt.Println("Omodos AI initialized. Run 'ggo --help' for commands.")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Define the --configdir flag globally for all commands
	rootCmd.PersistentFlags().StringVar(&cfgDirFlag, "configdir", "", "config directory (default is $GGO_CONFIG_DIR or ~/.ggo)")

	// Bind the flag to Viper so it overrides environment variables
	viper.BindPFlag("configdir", rootCmd.PersistentFlags().Lookup("configdir"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// 1. Resolve configuration directory in order of precedence: Flag -> Env -> Default
	configDir := viper.GetString("configdir")
	if configDir == "" {
		configDir = os.Getenv("GGO_CONFIG_DIR")
	}
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Error: Could not resolve user home directory:", err)
			os.Exit(1)
		}
		configDir = filepath.Join(home, ".ggo")
	}

	// Ensure the base vault exists
	os.MkdirAll(configDir, 0755)

	// 2. Configure Viper to use this resolved directory
	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv() // Read in environment variables that match

	// 3. Load Global Variables into memory (Global struct lives in config.go)
	if err := viper.ReadInConfig(); err == nil {
		Global.SourceDir = viper.GetString("source_dir")
		Global.ChunksDir = viper.GetString("chunks_dir")
		Global.IngestDir = viper.GetString("ingest_dir")
		Global.ExportDate = viper.GetString("export_date")
	}
}
