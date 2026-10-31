// Copyright 2025-2026 Peter Braam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	var activeProject string

	// 1. Explicit project override passed via CLI flag (-p / --project) or parameter
	if overrideProject != "" {
		if path, exists := GlobalSt.Projects[overrideProject]; exists {
			activeProject = overrideProject
			os.Chdir(path)
		} else {
			fmt.Printf("[Error] Project '%s' not found in ledger. Enrolled projects:\n", overrideProject)
			for p, path := range GlobalSt.Projects {
				fmt.Printf("  - %-15s [%s]\n", p, path)
			}
			os.Exit(1)
		}
	} else if GlobalSt.LastProject != "" {
		// 2. Fall back strictly to last explicitly selected project in state.json
		if path, exists := GlobalSt.Projects[GlobalSt.LastProject]; exists {
			activeProject = GlobalSt.LastProject
			os.Chdir(path)
		}
	}

	// 3. Halt cleanly if no explicit project context is available
	if activeProject == "" {
		fmt.Println("[Error] No active project specified and no valid last_project found.")
		fmt.Println("        Explicitly specify a project using: ggo -p <project_name>")
		if len(GlobalSt.Projects) > 0 {
			fmt.Println("\nEnrolled projects:")
			for p, path := range GlobalSt.Projects {
				fmt.Printf("  - %-15s [%s]\n", p, path)
			}
		}
		os.Exit(1)
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
