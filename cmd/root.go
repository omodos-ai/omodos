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
		RunREPL()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgDirFlag, "configdir", "", "config directory (default is $GGO_CONFIG_DIR or ~/.ggo)")
	viper.BindPFlag("configdir", rootCmd.PersistentFlags().Lookup("configdir"))
}

func initConfig() {
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

	os.MkdirAll(configDir, 0755)

	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		Global.RecordsDir = viper.GetString("records_dir")
		Global.ChunksDir = viper.GetString("chunks_dir")
		Global.IngestDir = viper.GetString("ingest_dir")
		Global.ExportDate = viper.GetString("export_date")
	}
}
