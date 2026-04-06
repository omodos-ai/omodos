package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var ForceNew bool // Bound to the -n / --new flag in root.go

type GlobalConfig struct {
	SourceDir   string            `mapstructure:"source_dir"`
	ChunksDir   string            `mapstructure:"chunks_dir"`
	IngestDir   string            `mapstructure:"ingest_dir"`
	ExportDate  string            `mapstructure:"export_date"`
	Projects    map[string]string `mapstructure:"projects"`
	LastProject string            `mapstructure:"last_project"` // Tracks the last active project
}

type ProjectConfig struct {
	ProjectName  string   `mapstructure:"project_name"`
	SourceDir    string   `mapstructure:"source_dir"`
	CodeDirs     []string `mapstructure:"code_dirs"`
	IncludedData []string `mapstructure:"included_data"`
	ModelName    string   `mapstructure:"model_name"`
}

var (
	Global       GlobalConfig
	Project      ProjectConfig
	GlobalViper  *viper.Viper
	ProjectViper *viper.Viper
)

func InitConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	GlobalViper = viper.New()
	GlobalViper.SetConfigFile(filepath.Join(home, ".ggo", "config.yaml"))
	GlobalViper.SetConfigType("yaml")
	if err := GlobalViper.ReadInConfig(); err == nil {
		GlobalViper.Unmarshal(&Global)
	}

	cwd, _ := os.Getwd()
	var activeProject string
	var longestMatch int

	// 1. FAST RESUME: If we have a last project and the user didn't request a --new session
	if !ForceNew && Global.LastProject != "" && Global.Projects != nil {
		if path, exists := Global.Projects[Global.LastProject]; exists {
			activeProject = Global.LastProject
			// Teleport the user's working directory back to the project root
			os.Chdir(path)
		}
	}

	// 2. PATH RESOLUTION: If --new was passed, or no last project exists, resolve via CWD
	if activeProject == "" && Global.Projects != nil {
		for name, path := range Global.Projects {
			if strings.HasPrefix(cwd, path) {
				if len(path) > longestMatch {
					longestMatch = len(path)
					activeProject = name
				}
			}
		}
	}

	if activeProject != "" {
		// Persist the active project as the new LastProject
		Global.LastProject = activeProject
		GlobalViper.Set("last_project", activeProject)
		GlobalViper.WriteConfigAs(filepath.Join(home, ".ggo", "config.yaml"))

		ProjectViper = viper.New()
		ProjectViper.SetConfigFile(filepath.Join(home, ".ggo", "ggoai-"+activeProject, "project.yaml"))
		ProjectViper.SetConfigType("yaml")

		if err := ProjectViper.ReadInConfig(); err == nil {
			ProjectViper.Unmarshal(&Project)
		}
	}
}
