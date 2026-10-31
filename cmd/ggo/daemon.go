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

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "(Deprecated) ggo now uses a unified single-process event loop.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("[Info] The standalone daemon process has been deprecated.")
		fmt.Println("ggo now runs a unified, single-process event loop that automatically handles pipes and terminal input simultaneously.")
		fmt.Println("Please run 'ggo' normally to start the hub.")
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
