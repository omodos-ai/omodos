package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
)

var lastAPIResponse string

func RunREPL() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[!] Caught termination signal. Cleaning up...")
		CleanupTimeMachine()
		os.Exit(0)
	}()

	var completer = readline.NewPrefixCompleter(
		readline.PcItem("quit"),
		readline.PcItem("exit"),
		readline.PcItem("clear"),
		readline.PcItem("list", readline.PcItem("projects")),
		readline.PcItem("show", readline.PcItem("global"), readline.PcItem("project")),
		readline.PcItem("set",
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
		if line == "" {
			continue
		}

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
			handleList(parts)
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
// ... [keep handleList, handleShow, handleSet, expandTilde, listFiles the same as before] ...
func handleList(parts []string) {
	if len(parts) < 2 || parts[1] != "projects" {
		fmt.Println("Usage: list projects")
		return
	}
	fmt.Println("\n--- Tracked ggo Projects ---")
	if len(Global.Projects) == 0 {
		fmt.Println("No projects tracked yet.")
	} else {
		for name, path := range Global.Projects {
			fmt.Printf("- %-15s [%s]\n", name, path)
		}
	}
	fmt.Println()
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
	if strings.HasSuffix(field, "_dir") || strings.HasSuffix(field, "path") {
		expanded := expandTilde(value)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			fmt.Printf("[Error] Path does not exist: %s\n", expanded)
			return
		}
		value = expanded
	}
	isGlobal := GlobalViper.IsSet(field) || field == "export_date" || field == "source_dir" || field == "chunks_dir" || field == "ingest_dir"
	isProject := ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd"

	if isGlobal {
		GlobalViper.Set(field, value)
		GlobalViper.WriteConfig()
		InitConfig()
	} else if isProject {
		ProjectViper.Set(field, value)
		ProjectViper.WriteConfigAs(filepath.Join(".ggo", "project.yaml"))
		InitConfig()
	}
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if path == "~" { return home }
		if strings.HasPrefix(path, "~/") { return filepath.Join(home, path[2:]) }
	}
	return path
}

func listFiles(line string) []string { return []string{} } // Stub to save space, keep your existing logic
