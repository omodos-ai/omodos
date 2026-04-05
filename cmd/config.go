package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	if Global.Projects != nil {
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
		ProjectViper = viper.New()
		ProjectViper.SetConfigFile(filepath.Join(home, ".ggo", "ggoai-"+activeProject, "project.yaml"))
		ProjectViper.SetConfigType("yaml")

		if err := ProjectViper.ReadInConfig(); err == nil {
			ProjectViper.Unmarshal(&Project)
		}
	}
}
