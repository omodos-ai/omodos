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

package raggo

import (
	"fmt"
	"os"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
)

var projectFlag string

func init() {
	rootCmd.PersistentFlags().StringVarP(&projectFlag, "project", "p", "", "Explicitly override the project context")
}

var rootCmd = &cobra.Command{
	Use:   "raggo",
	Short: "Omodos AI Knowledge Manager",
	Long:  "raggo ingests, extracts, and chunks AI data exports for NotebookLM and Vertex.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize only the global config to prevent project enrollment traps
		core.InitGlobalConfig()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
