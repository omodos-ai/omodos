package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	GlobalCfg  *pb.GlobalConfig
	GlobalSt   *pb.GlobalState
	ProjectCfg *pb.ProjectConfig
	ProjectSt  *pb.ProjectState

	GlobalViper  *viper.Viper
	ProjectViper *viper.Viper
)

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

func InitGlobalConfig() {
	configDir := GetConfigDir()
	os.MkdirAll(configDir, 0755)

	GlobalCfg = &pb.GlobalConfig{}
	GlobalViper = viper.New()
	GlobalViper.SetConfigFile(filepath.Join(configDir, "config.yaml"))
	GlobalViper.SetConfigType("yaml")
	if err := GlobalViper.ReadInConfig(); err == nil {
		GlobalCfg.RecordsDir = ExpandTilde(GlobalViper.GetString("records_dir"))
		GlobalCfg.ChunksDir = ExpandTilde(GlobalViper.GetString("chunks_dir"))
		GlobalCfg.IngestDir = ExpandTilde(GlobalViper.GetString("ingest_dir"))
	}

	GlobalSt = &pb.GlobalState{Projects: make(map[string]string)}
	statePath := filepath.Join(configDir, "state.json")
	if bytes, err := os.ReadFile(statePath); err == nil {
		protojson.Unmarshal(bytes, GlobalSt)
	}
	if GlobalSt.Projects == nil {
		GlobalSt.Projects = make(map[string]string)
	}
}

func InitConfig(overrideProject string) {
	InitGlobalConfig()

	cwd, _ := os.Getwd()
	var activeProject string

	if overrideProject != "" {
		if path, exists := GlobalSt.Projects[overrideProject]; exists {
			activeProject = overrideProject
			os.Chdir(path)
		} else {
			fmt.Printf("[Error] Project '%s' not found in ledger. Known projects:\n", overrideProject)
			for p := range GlobalSt.Projects {
				fmt.Printf("  - %s\n", p)
			}
			os.Exit(1)
		}
	}

	if activeProject == "" {
		for name, path := range GlobalSt.Projects {
			if cwd == path {
				activeProject = name
				break
			}
		}
	}

	if activeProject == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			activeProject = "global_headless"
		} else {
			basename := filepath.Base(cwd)
			fmt.Printf("\n\033[33m[?] Directory not enrolled:\033[0m %s\n", cwd)
			fmt.Printf("Adopt this directory as new project '%s'? [Y/n] ", basename)

			reader := bufio.NewReader(os.Stdin)
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(strings.ToLower(resp))

			if resp == "" || resp == "y" || resp == "yes" {
				activeProject = basename
				GlobalSt.Projects[activeProject] = cwd
				fmt.Printf("\033[32m[✓] Adopted new project:\033[0m %s\n\n", activeProject)
			} else {
				fmt.Println("\n[!] Boot sequence aborted.")
				fmt.Println("    To join an existing project, use: ggo -p <name>")
				os.Exit(0)
			}
		}
	}

	GlobalSt.LastProject = activeProject
	SaveGlobalState()

	ProjectCfg = &pb.ProjectConfig{ProjectName: activeProject}
	projVault := GetProjectVaultDir(activeProject)
	os.MkdirAll(projVault, 0755)

	ProjectSt = &pb.ProjectState{}
	projStatePath := filepath.Join(projVault, "state.json")
	if bytes, err := os.ReadFile(projStatePath); err == nil {
		protojson.Unmarshal(bytes, ProjectSt)
	}

	ProjectViper = viper.New()
	ProjectViper.SetConfigFile(filepath.Join(projVault, "project.yaml"))
	ProjectViper.SetConfigType("yaml")
	if err := ProjectViper.ReadInConfig(); err == nil {
		ProjectCfg.CodeDirs = ProjectViper.GetStringSlice("code_dirs")
		ProjectCfg.ModelName = ProjectViper.GetString("model_name")
		ProjectCfg.Role = ProjectViper.GetString("role")

		// BUG FIX: Hydrate the in-memory array from the persistent config on boot
		ProjectSt.IncludedData = ProjectViper.GetStringSlice("included_data")
	}
	ProjectCfg.ProjectName = activeProject
}

func SaveGlobalState() {
	bytes, _ := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: false}.Marshal(GlobalSt)
	os.WriteFile(filepath.Join(GetConfigDir(), "state.json"), bytes, 0644)
}

func SaveProjectState() {
	projVault := GetProjectVaultDir(ProjectCfg.ProjectName)
	bytes, _ := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: false}.Marshal(ProjectSt)
	os.WriteFile(filepath.Join(projVault, "state.json"), bytes, 0644)
}

func GetConfigDir() string {
	if envDir := os.Getenv("GGO_CONFIG_DIR"); envDir != "" {
		return envDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ggo")
}

func GetProjectVaultDir(projectName string) string {
	return filepath.Join(GetConfigDir(), "ggoai-"+projectName)
}
