package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
)

var ForceNew bool

// The strictly typed, segregated configuration structs
var (
	GlobalCfg  *pb.GlobalConfig
	GlobalSt   *pb.GlobalState
	ProjectCfg *pb.ProjectConfig
	ProjectSt  *pb.ProjectState

	GlobalViper  *viper.Viper
	ProjectViper *viper.Viper
)

// ExpandTilde safely resolves ~ to the absolute user home directory
func ExpandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if path == "~" {
			return home
		}
		if strings.HasPrefix(path, "~/") {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func InitConfig() {
	configDir := GetConfigDir()
	os.MkdirAll(configDir, 0755)

	// 1. Load Global Config (Human Preferences - YAML)
	GlobalCfg = &pb.GlobalConfig{}
	GlobalViper = viper.New()
	GlobalViper.SetConfigFile(filepath.Join(configDir, "config.yaml"))
	GlobalViper.SetConfigType("yaml")
	if err := GlobalViper.ReadInConfig(); err == nil {
		// Enforce Tilde Expansion on all paths
		GlobalCfg.RecordsDir = ExpandTilde(GlobalViper.GetString("records_dir"))
		GlobalCfg.ChunksDir = ExpandTilde(GlobalViper.GetString("chunks_dir"))
		GlobalCfg.IngestDir = ExpandTilde(GlobalViper.GetString("ingest_dir"))
	}

	// 2. Load Global State (Machine Telemetry - JSON)
	GlobalSt = &pb.GlobalState{Projects: make(map[string]string)}
	statePath := filepath.Join(configDir, "state.json")
	if bytes, err := os.ReadFile(statePath); err == nil {
		protojson.Unmarshal(bytes, GlobalSt)
	}

	cwd, _ := os.Getwd()
	var activeProject string
	var longestMatch int

	// 1. Detect project purely based on the physical directory we are standing in
	if GlobalSt.Projects != nil {
		for name, path := range GlobalSt.Projects {
			if strings.HasPrefix(cwd, path) {
				if len(path) > longestMatch {
					longestMatch = len(path)
					activeProject = name
				}
			}
		}
	}

	// 2. Fallback if unregistered
	if activeProject == "" {
		activeProject = "global"
	}

	// Persist the active project state instantly
	GlobalSt.LastProject = activeProject
	SaveGlobalState()

	// 3. Load Project Config (Human Preferences - YAML)
	ProjectCfg = &pb.ProjectConfig{ProjectName: activeProject}
	projVault := GetProjectVaultDir(activeProject)
	os.MkdirAll(projVault, 0755)

	ProjectViper = viper.New()
	ProjectViper.SetConfigFile(filepath.Join(projVault, "project.yaml"))
	ProjectViper.SetConfigType("yaml")
	if err := ProjectViper.ReadInConfig(); err == nil {
		ProjectCfg.CodeDirs = ProjectViper.GetStringSlice("code_dirs")
		ProjectCfg.ModelName = ProjectViper.GetString("model_name")
		ProjectCfg.Temperature = ProjectViper.GetFloat64("temperature")
		ProjectCfg.TopP = ProjectViper.GetFloat64("top_p")
		ProjectCfg.SafetyLevel = ProjectViper.GetString("safety_level")
		ProjectCfg.Role = ProjectViper.GetString("role")
		ProjectCfg.RecordsDir = ExpandTilde(ProjectViper.GetString("records_dir"))
	}
	ProjectCfg.ProjectName = activeProject // Enforce structural integrity

	// 4. Load Project State (Machine Telemetry - JSON)
	ProjectSt = &pb.ProjectState{}
	projStatePath := filepath.Join(projVault, "state.json")
	if bytes, err := os.ReadFile(projStatePath); err == nil {
		protojson.Unmarshal(bytes, ProjectSt)
	}
}

// --- State Persisters (Bypasses Viper to protect Config) ---

func SaveGlobalState() {
	bytes, err := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: false}.Marshal(GlobalSt)
	if err == nil {
		os.WriteFile(filepath.Join(GetConfigDir(), "state.json"), bytes, 0644)
	}
}

func SaveProjectState() {
	projVault := GetProjectVaultDir(ProjectCfg.ProjectName)
	os.MkdirAll(projVault, 0755)
	bytes, err := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: false}.Marshal(ProjectSt)
	if err == nil {
		os.WriteFile(filepath.Join(projVault, "state.json"), bytes, 0644)
	}
}

// --- Path Helpers ---

func GetConfigDir() string {
	if envDir := os.Getenv("GGO_CONFIG_DIR"); envDir != "" {
		return envDir
	}
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
