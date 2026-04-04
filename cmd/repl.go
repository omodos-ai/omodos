package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
)

var lastAPIResponse string

func RunREPL() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if Runtime.IsThinking && sig == os.Interrupt {
				continue
			}
			fmt.Println("\n[!] Caught termination signal. Cleaning up...")
			CleanupTimeMachine()
			os.Exit(0)
		}
	}()

	var completer = readline.NewPrefixCompleter(
		readline.PcItem("quit"),
		readline.PcItem("exit"),
		readline.PcItem("clear"),
		readline.PcItem("list", readline.PcItem("projects"), readline.PcItem("models")),
		readline.PcItem("show", readline.PcItem("global"), readline.PcItem("project")),
		readline.PcItem("set",
			readline.PcItem("model"),
			readline.PcItem("source_dir"),
			readline.PcItem("chunks_dir"),
			readline.PcItem("ingest_dir"),
			readline.PcItem("export_date"),
			readline.PcItem("project_name"),
			readline.PcItem("shell_cmd"),
			readline.PcItem("diff_cmd"),
		),
		readline.PcItem("inc", readline.PcItemDynamic(listFiles)),
		readline.PcItem("p"),
		readline.PcItem("ps"),
		readline.PcItem("cr"),
		readline.PcItem("cw"),
		readline.PcItem("gl"),
		readline.PcItem("gd"),
		readline.PcItem("gs"),
	)

	historyPath := filepath.Join(".ggo", "ggo_history")
	l, err := readline.NewEx(&readline.Config{
		Prompt:            "\033[36mggo>\033[0m ",
		HistoryFile:       historyPath,
		AutoComplete:      completer,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		fmt.Printf("Error initializing REPL: %v\n", err)
		os.Exit(1)
	}
	defer l.Close()

	ctx := context.Background()
	fmt.Printf("Initializing Gemini API... ")
	err = InitGemini(ctx)
	if err != nil {
		fmt.Printf("\n[Warning] AI disabled: %v\n", err)
	} else {
		fmt.Printf("[OK]\n")
	}

	fmt.Printf("Welcome to ggo [%s]. Type 'quit' to exit.\n", Project.ProjectName)

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" { continue }

		if strings.HasPrefix(line, "!") {
			fmt.Println("-> Shell execution stubbed.")
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "quit", "exit":
			fmt.Println("Flushing ledger and exiting...")
			CleanupTimeMachine()
			os.Exit(0)
		case "clear":
			fmt.Println("-> Stub: Clearing active chat context")
		case "list":
			handleList(ctx, parts)
		case "show":
			handleShow(parts)
		case "set":
			handleSet(parts)
		case "inc":
			handleInc(ctx, parts)
		case "cr":
			handleCR()
		case "cw":
			handleCW(parts)
		case "gl":
			handleGL(parts)
		case "gd":
			handleGD(parts)
		case "gs":
			handleGS(parts)
		case "p":
			handleP(ctx, l)
		case "ps":
			handlePS(ctx, l)
		default:
			sendAndRender(ctx, line)
		}
	}
	CleanupTimeMachine()
}

func handleList(ctx context.Context, parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: list <projects|models>")
		return
	}
	target := parts[1]
	if target == "projects" {
		fmt.Println("\n--- Tracked ggo Projects ---")
		if len(Global.Projects) == 0 {
			fmt.Println("No projects tracked yet.")
		} else {
			for name, path := range Global.Projects {
				fmt.Printf("- %-15s [%s]\n", name, path)
			}
		}
		fmt.Println()
	} else if target == "models" {
		FetchAndListModels(ctx)
	} else {
		fmt.Println("Unknown list target. Use 'projects' or 'models'.")
	}
}

func handleShow(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: show <global|project>")
		return
	}
	target := parts[1]
	if target == "global" {
		fmt.Println("\n--- Global Config ---")
		for k, v := range GlobalViper.AllSettings() {
			fmt.Printf("%s: %v\n", k, v)
		}
		fmt.Println()
	} else if target == "project" {
		fmt.Println("\n--- Project Config ---")
		for k, v := range ProjectViper.AllSettings() {
			fmt.Printf("%s: %v\n", k, v)
		}
		fmt.Println()
	} else {
		fmt.Println("Unknown show target. Use 'global' or 'project'.")
	}
}

func handleSet(parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: set <field> <value>")
		return
	}
	field := parts[1]
	value := strings.Join(parts[2:], " ")

	if field == "model" || field == "model_name" {
		field = "model_name"
		if idx, err := strconv.Atoi(value); err == nil {
			if idx > 0 && idx <= len(View.ModelList) {
				value = View.ModelList[idx-1]
			} else {
				fmt.Println("[Error] Invalid model number. Run 'list models' first.")
				return
			}
		}
	}

	if strings.HasSuffix(field, "_dir") || strings.HasSuffix(field, "path") {
		expanded := expandTilde(value)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			fmt.Printf("[Error] Path does not exist: %s\n", expanded)
			fmt.Println("Set operation aborted.")
			return
		}
		value = expanded
	}

	isGlobal := GlobalViper.IsSet(field) || field == "export_date" || field == "source_dir" || field == "chunks_dir" || field == "ingest_dir"
	isProject := ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd" || field == "model_name"

	// Wrap the config change in our universal transaction engine
	ExecuteTransaction("set_"+field, func(timestamp string, now time.Time) error {
		if isGlobal {
			GlobalViper.Set(field, value)
			if err := GlobalViper.WriteConfig(); err != nil {
				fmt.Printf("[Error] Failed to save global config: %v\n", err)
			} else {
				fmt.Printf("[✓] Global updated: %s = %s\n", field, value)
				InitConfig()
			}
		} else if isProject {
			ProjectViper.Set(field, value)
			os.MkdirAll(".ggo", 0755)
			if err := ProjectViper.WriteConfigAs(filepath.Join(".ggo", "project.yaml")); err != nil {
				fmt.Printf("[Error] Failed to save project config: %v\n", err)
				return err
			}
			fmt.Printf("[✓] Project updated: %s = %s\n", field, value)
			InitConfig()
			
			if field == "model_name" {
				UpdateChatModel()
				fmt.Println("[✓] AI Engine hot-swapped successfully.")
			}
		} else {
			fmt.Printf("[Error] Unknown field: %s\n", field)
		}
		return nil
	})
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" { return home }
			if strings.HasPrefix(path, "~/") { return filepath.Join(home, path[2:]) }
		}
	}
	return path
}

func listFiles(line string) []string {
	search := line + "*"
	if line == "" { search = "*" }
	matches, err := filepath.Glob(search)
	if err != nil { return []string{} }

	var completions []string
	for _, match := range matches {
		if stat, err := os.Stat(match); err == nil && stat.IsDir() {
			completions = append(completions, match+"/")
		} else {
			completions = append(completions, match)
		}
	}
	return completions
}
