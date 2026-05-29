package raggo

import (
	"fmt"
	"os"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "raggo",
	Short: "Omodos AI Knowledge Manager",
	Long:  "raggo ingests, extracts, and chunks AI data exports for NotebookLM and Vertex.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.InitConfig()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
