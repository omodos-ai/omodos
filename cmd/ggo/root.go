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

package ggo

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
	Use:   "ggo",
	Short: "Omodos AI Coding Assistant",
	Long:  "ggo manages Git-aware AI coding interactions, Time Machine transactions, and context management.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.InitConfig(projectFlag)
	},
	Run: func(cmd *cobra.Command, args []string) {
		RunREPL()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
