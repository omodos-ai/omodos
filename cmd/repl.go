package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

func RunREPL() {
	var completer = readline.NewPrefixCompleter(
		readline.PcItem("quit"),
		readline.PcItem("exit"),
		readline.PcItem("clear"),
		readline.PcItem("list", readline.PcItem("projects")),
		readline.PcItem("show", readline.PcItem("global"), readline.PcItem("project")),
		readline.PcItem("set",
			// Global fields
			readline.PcItem("source_dir"),
			readline.PcItem("chunks_dir"),
			readline.PcItem("ingest_dir"),
			readline.PcItem("export_date"),
			// Project fields
			readline.PcItem("project_name"),
			readline.PcItem("shell_cmd"),
			readline.PcItem("diff_cmd"),
		),
		readline.PcItem("inc", readline.PcItemDynamic(listFiles)),
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

	fmt.Printf("Welcome to ggo [%s]. Type 'quit' to exit.\n", Project.ProjectName)

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 { break } else { continue }
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" { continue }

		if strings.HasPrefix(line, "!") {
			fmt.Println("-> Shell execution stubbed.")
			continue
		}

		// Reserved Commands Engine
		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "quit", "exit":
			fmt.Println("Flushing ledger and exiting...")
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
			if len(parts) < 2 {
				fmt.Println("Usage: inc <path>")
			} else {
				fmt.Printf("-> Stub: Including %s into context\n", parts[1])
			}
		default:
			// Everything else goes to Gemini
			fmt.Printf("-> Gemini Prompt stubbed: %s\n", line)
		}
	}
}

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

	// 1. Path Validation: Complain before damage is done
	if strings.HasSuffix(field, "_dir") || strings.HasSuffix(field, "path") {
		if _, err := os.Stat(value); os.IsNotExist(err) {
			fmt.Printf("[Error] Path does not exist: %s\n", value)
			fmt.Println("Set operation aborted.")
			return
		}
	}

	// 2. Route and Save to correct Viper instance
	isGlobal := GlobalViper.IsSet(field) || field == "export_date" || field == "source_dir" || field == "chunks_dir" || field == "ingest_dir"
	isProject := ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd"

	if isGlobal {
		GlobalViper.Set(field, value)
		if err := GlobalViper.WriteConfig(); err != nil {
			fmt.Printf("[Error] Failed to save global config: %v\n", err)
		} else {
			fmt.Printf("[✓] Global updated: %s = %s\n", field, value)
			InitConfig() // Reload structs
		}
	} else if isProject {
		ProjectViper.Set(field, value)
		if err := ProjectViper.WriteConfig(); err != nil {
			// WriteConfig fails if the file doesn't exist yet, fallback to WriteConfigAs
			err = ProjectViper.WriteConfigAs(filepath.Join(".ggo", "project.yaml"))
			if err != nil {
				fmt.Printf("[Error] Failed to save project config: %v\n", err)
				return
			}
		}
		fmt.Printf("[✓] Project updated: %s = %s\n", field, value)
		InitConfig() // Reload structs
	} else {
		fmt.Printf("[Error] Unknown field: %s\n", field)
	}
}

func listFiles(line string) []string {
	return []string{}
}
