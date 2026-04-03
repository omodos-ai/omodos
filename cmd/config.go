package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type GlobalConfig struct {
	APIKeyPath string            `mapstructure:"api_key_path"`
	SourceDir  string            `mapstructure:"source_dir"`
	ChunksDir  string            `mapstructure:"chunks_dir"`
	IngestDir  string            `mapstructure:"ingest_dir"`
	ExportDate string            `mapstructure:"export_date"`
	Projects   map[string]string `mapstructure:"projects"` // Tracks Name -> Path
}

type ProjectConfig struct {
	ProjectName  string   `mapstructure:"project_name"`
	CodeDirs     []string `mapstructure:"code_dirs"`
	OtherDirs    []string `mapstructure:"other_dirs"`
	IncludedData []string `mapstructure:"included_data"`
	ShellCmd     string   `mapstructure:"shell_cmd"`
	DiffCmd      string   `mapstructure:"diff_cmd"`
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
		home = "."
	}

	// --- 1. Global Configuration ---
	GlobalViper = viper.New()
	GlobalViper.SetConfigName("config")
	GlobalViper.SetConfigType("yaml")
	GlobalViper.AddConfigPath(filepath.Join(home, ".ggo"))

	GlobalViper.SetDefault("source_dir", filepath.Join(home, "ai", "chats", "source"))
	GlobalViper.SetDefault("chunks_dir", filepath.Join(home, "ai", "chats", "chunks"))
	GlobalViper.SetDefault("ingest_dir", filepath.Join(home, "ai", "chats", "ingest"))
	GlobalViper.SetDefault("export_date", "2026-03-27")
	GlobalViper.SetDefault("projects", make(map[string]string))

	if err := GlobalViper.ReadInConfig(); err == nil {
		_ = GlobalViper.Unmarshal(&Global)
	} else {
		_ = GlobalViper.Unmarshal(&Global)
	}

	// Ensure the map is initialized if missing
	if Global.Projects == nil {
		Global.Projects = make(map[string]string)
	}

	// --- 2. Project Configuration ---
	ProjectViper = viper.New()
	ProjectViper.SetConfigName("project")
	ProjectViper.SetConfigType("yaml")
	ProjectViper.AddConfigPath(".ggo")

	ProjectViper.SetDefault("shell_cmd", "bash")
	ProjectViper.SetDefault("diff_cmd", "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'")

	projectLoaded := false
	if err := ProjectViper.ReadInConfig(); err == nil {
		_ = ProjectViper.Unmarshal(&Project)
		projectLoaded = true
	} else {
		_ = ProjectViper.Unmarshal(&Project)
	}

	// --- 3. Validate Symlinks ---
	if Global.APIKeyPath != "" {
		if _, err := os.Stat(Global.APIKeyPath); os.IsNotExist(err) {
			fmt.Printf("[Warning] API key file missing or dangling at: %s\n", Global.APIKeyPath)
		}
	}

	// --- 4. Auto-Track Project ---
	if projectLoaded && Project.ProjectName != "" {
		cwd, _ := os.Getwd()
		if Global.Projects[Project.ProjectName] != cwd {
			Global.Projects[Project.ProjectName] = cwd
			GlobalViper.Set("projects", Global.Projects)
			_ = GlobalViper.WriteConfig()
		}
	}
}
