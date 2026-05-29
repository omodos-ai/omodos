package ggo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/spf13/cobra"
)

// sendToPipeIfRunning attempts a non-blocking write to the FIFO pipe.
// If no reader (REPL/Emacs) is actively listening, the OS instantly returns an error
// and we safely fall back to executing the command directly in the current terminal.
func sendToPipeIfRunning(pipeName, payload string) bool {
	if core.ProjectCfg == nil || core.ProjectCfg.ProjectName == "" {
		return false
	}
	pipePath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "run", pipeName)

	f, err := os.OpenFile(pipePath, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return false
	}
	defer f.Close()

	f.WriteString(payload + "\n")
	return true
}

func init() {
	replCommands := []string{
		"list", "add", "show", "set", "context-include", "inc", "git-add-repo",
		"gr", "git-remote", "cc", "ps", "cr", "cw", "wf", "write-files",
		"gl", "gd", "gs",
	}

	for _, cmdName := range replCommands {
		name := cmdName // Capture for closure
		cmd := &cobra.Command{
			Use:                name,
			Short:              fmt.Sprintf("Execute '%s' directly or send to active REPL", name),
			DisableFlagParsing: true,
			Run: func(cmd *cobra.Command, args []string) {
				fullCmd := name
				if len(args) > 0 {
					fullCmd += " " + strings.Join(args, " ")
				}

				if sent := sendToPipeIfRunning("control.pipe", fullCmd); sent {
					fmt.Printf("[\033[35mSent to active ggo session\033[0m] %s\n", fullCmd)
					return
				}

				// Fallback to synchronous execution
				ctx := context.Background()
				core.InitGemini(ctx) // <-- FIX: Spin up the API for standalone CLI usage
				executeCommand(ctx, fullCmd, nil, false)
			},
		}
		rootCmd.AddCommand(cmd)
	}

	promptCmd := &cobra.Command{
		Use:                "prompt <text...>",
		Short:              "Send a prompt directly to the active chat",
		Aliases:            []string{"ask", "q"},
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			fullPrompt := strings.Join(args, " ")

			if sent := sendToPipeIfRunning("prompt.pipe", fullPrompt); sent {
				fmt.Printf("[\033[35mSent to active ggo session\033[0m] Prompt dispatched.\n")
				return
			}

			// Fallback to synchronous execution
			ctx := context.Background()
			core.InitGemini(ctx) // <-- FIX: Spin up the API for standalone CLI usage
			executeCommand(ctx, fullPrompt, nil, false)
		},
	}
	rootCmd.AddCommand(promptCmd)
}
