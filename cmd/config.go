package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var ForceNew bool

type GlobalConfig struct {
	RecordsDir  string            `mapstructure:"records_dir"`
	ChunksDir   string            `mapstructure:"chunks_dir"`
	IngestDir   string            `mapstructure:"ingest_dir"`
	ExportDate  string            `mapstructure:"export_date"`
	Projects    map[string]string `mapstructure:"projects"`
	LastProject string            `mapstructure:"last_project"`
}

type ProjectConfig struct {
	ProjectName  string   `mapstructure:"project_name"`
	RecordsDir   string   `mapstructure:"records_dir"`
	CodeDirs     []string `mapstructure:"code_dirs"`
	IncludedData []string `mapstructure:"included_data"`
	ModelName    string   `mapstructure:"model_name"`
	Temperature  float64  `mapstructure:"temperature"`
	TopP         float64  `mapstructure:"top_p"`
	SafetyLevel  string   `mapstructure:"safety_level"`
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

	if !ForceNew && Global.LastProject != "" && Global.Projects != nil {
		if path, exists := Global.Projects[Global.LastProject]; exists {
			activeProject = Global.LastProject
			os.Chdir(path)
		}
	}

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

	// --- THE FIX: Fallback Workspace ---
	// If the user runs ggo outside of a tracked directory, guarantee
	// a default "global" workspace so ProjectViper is never nil.
	if activeProject == "" {
		activeProject = "global"
	}

	Global.LastProject = activeProject
	GlobalViper.Set("last_project", activeProject)
	GlobalViper.WriteConfigAs(filepath.Join(home, ".ggo", "config.yaml"))

	ProjectViper = viper.New()
	ProjectViper.SetConfigFile(filepath.Join(home, ".ggo", "ggoai-"+activeProject, "project.yaml"))
	ProjectViper.SetConfigType("yaml")

	if err := ProjectViper.ReadInConfig(); err == nil {
		ProjectViper.Unmarshal(&Project)
	}

	// Force the struct to match the active project name
	Project.ProjectName = activeProject
}

// Add this helper to cmd/config.go
func GetConfigDir() string {
	// 1. Check environment variable first
	if envDir := os.Getenv("GGO_CONFIG_DIR"); envDir != "" {
		return envDir
	}
	// 2. Fallback to standard home directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[Fatal] Could not resolve home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".ggo")
}

func GetProjectVaultDir(projectName string) string {
	return filepath.Join(GetConfigDir(), "ggoai-"+projectName)
}
