package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type GlobalConfig struct {
	APIKeyPath  string            `mapstructure:"api_key_path"`
	SourceDir   string            `mapstructure:"source_dir"`
	ChunksDir   string            `mapstructure:"chunks_dir"`
	IngestDir   string            `mapstructure:"ingest_dir"`
	ExportDate  string            `mapstructure:"export_date"`
	Editor      string            `mapstructure:"editor"`
	QuickEditor string            `mapstructure:"quick_editor"`
	Viewer      string            `mapstructure:"viewer"`
	DiffCmd     string            `mapstructure:"diff_cmd"`
	Projects    map[string]string `mapstructure:"projects"` // Tracks Name -> Absolute Path
}

type ProjectConfig struct {
	ProjectName  string   // Dynamically inferred from global projects map
	CodeDirs     []string `mapstructure:"code_dirs"`
	OtherDirs    []string `mapstructure:"other_dirs"`
	IncludedData []string `mapstructure:"included_data"`
	ShellCmd     string   `mapstructure:"shell_cmd"`
	DiffCmd      string   `mapstructure:"diff_cmd"` // Project-level override
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

	// Ensure the base central directory exists
	os.MkdirAll(filepath.Join(home, ".ggo"), 0755)

	// --- 1. Global Configuration ---
	GlobalViper = viper.New()
	GlobalViper.SetConfigName("config")
	GlobalViper.SetConfigType("yaml")
	GlobalViper.AddConfigPath(filepath.Join(home, ".ggo"))

	GlobalViper.SetDefault("source_dir", filepath.Join(home, "ai", "chats", "source"))
	GlobalViper.SetDefault("chunks_dir", filepath.Join(home, "ai", "chats", "chunks"))
	GlobalViper.SetDefault("ingest_dir", filepath.Join(home, "ai", "chats", "ingest"))
	GlobalViper.SetDefault("export_date", "2026-03-27")
	GlobalViper.SetDefault("editor", "emacs")
	GlobalViper.SetDefault("quick_editor", "vi")
	GlobalViper.SetDefault("viewer", "less")
	GlobalViper.SetDefault("diff_cmd", "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'")
	GlobalViper.SetDefault("projects", make(map[string]string))

	// Read or create the global config
	if err := GlobalViper.ReadInConfig(); err == nil {
		_ = GlobalViper.Unmarshal(&Global)
	} else {
		// Force create the file if missing to prevent nil map issues
		GlobalViper.SafeWriteConfigAs(filepath.Join(home, ".ggo", "config.yaml"))
		_ = GlobalViper.Unmarshal(&Global)
	}

	if Global.Projects == nil {
		Global.Projects = make(map[string]string)
	}

	// --- 2. Project Detection (Reverse Lookup CWD) ---
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd) // Ensure absolute path for exact matching

	for name, path := range Global.Projects {
		absPath, _ := filepath.Abs(path)
		if absPath == cwd {
			Project.ProjectName = name
			break
		}
	}

	// --- 3. Project Configuration ---
	ProjectViper = viper.New()
	ProjectViper.SetConfigName("project")
	ProjectViper.SetConfigType("yaml")

	if Project.ProjectName != "" {
		projDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
		ProjectViper.AddConfigPath(projDir)
		ProjectViper.SetDefault("shell_cmd", "bash")
		// Fallback to global diff_cmd if project doesn't specify one
		ProjectViper.SetDefault("diff_cmd", Global.DiffCmd)

		if err := ProjectViper.ReadInConfig(); err == nil {
			_ = ProjectViper.Unmarshal(&Project)
		}
	}

	// --- 4. Validate Symlinks ---
	if Global.APIKeyPath != "" {
		if _, err := os.Stat(Global.APIKeyPath); os.IsNotExist(err) {
			fmt.Printf("[Warning] API key file missing or dangling at: %s\n", Global.APIKeyPath)
		}
	}
}
