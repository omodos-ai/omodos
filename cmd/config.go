package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type GlobalConfig struct {
	SourceDir  string            `mapstructure:"source_dir"`
	ChunksDir  string            `mapstructure:"chunks_dir"`
	IngestDir  string            `mapstructure:"ingest_dir"`
	ExportDate string            `mapstructure:"export_date"`
	Projects   map[string]string `mapstructure:"projects"`
}

type ProjectConfig struct {
	ProjectName  string   `mapstructure:"project_name"`
	SourceDir    string   `mapstructure:"source_dir"`
	CodeDirs     []string `mapstructure:"code_dirs"`
	IncludedData []string `mapstructure:"included_data"` // Restored!
	ModelName    string   `mapstructure:"model_name"`    // AI Model
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

	// Load Global Config
	GlobalViper = viper.New()
	GlobalViper.SetConfigFile(filepath.Join(home, ".ggo", "config.yaml"))
	GlobalViper.SetConfigType("yaml")
	if err := GlobalViper.ReadInConfig(); err == nil {
		GlobalViper.Unmarshal(&Global)
	}

	// Preserve identity to prevent amnesia during hot-reloads
	oldProjName := Project.ProjectName 

	// Load Project Config (Reads locally to figure out what project we are in)
	ProjectViper = viper.New()
	ProjectViper.SetConfigFile(filepath.Join(".ggo", "project.yaml"))
	ProjectViper.SetConfigType("yaml")
	if err := ProjectViper.ReadInConfig(); err == nil {
		ProjectViper.Unmarshal(&Project)
	}

	// Restore identity if the yaml file was missing it
	if Project.ProjectName == "" && oldProjName != "" {
		Project.ProjectName = oldProjName
		ProjectViper.Set("project_name", oldProjName)
	}

	// Default Model Fallback & Saving
	if Project.ModelName == "" {
		Project.ModelName = "gemini-3.1-pro-preview"
		ProjectViper.Set("model_name", Project.ModelName)
		
		if Project.ProjectName != "" {
			// Save to centralized LFS repo if the project is already named
			projDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
			os.MkdirAll(projDir, 0755)
			ProjectViper.WriteConfigAs(filepath.Join(projDir, "project.yaml"))
		} else {
			// Otherwise fallback to local initialization
			os.MkdirAll(".ggo", 0755)
			ProjectViper.WriteConfigAs(filepath.Join(".ggo", "project.yaml"))
		}
	}
}
