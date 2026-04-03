package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// Global state for the REPL to share across files
var lastAPIResponse string

func RunREPL() {
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

	// Initialize Gemini API
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
		case "p":
			handleP(ctx, l)
		case "ps":
			handlePS(ctx, l)
		default:
			// Single line prompt fallback
			sendAndRender(ctx, line)
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
	isProject := ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd"

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
		if err := ProjectViper.WriteConfig(); err != nil {
			err = ProjectViper.WriteConfigAs(filepath.Join(".ggo", "project.yaml"))
			if err != nil {
				fmt.Printf("[Error] Failed to save project config: %v\n", err)
				return
			}
		}
		fmt.Printf("[✓] Project updated: %s = %s\n", field, value)
		InitConfig()
	} else {
		fmt.Printf("[Error] Unknown field: %s\n", field)
	}
}

// expandTilde safely resolves "~" to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			if strings.HasPrefix(path, "~/") {
				return filepath.Join(home, path[2:])
			}
		}
	}
	return path
}

// listFiles provides dynamic, bash-like path auto-completion for the 'inc' command
func listFiles(line string) []string {
	// 'line' is the rest of the string after "inc ".
	// Split by spaces to isolate the exact token the user is currently typing
	words := strings.Split(line, " ")
	currentWord := words[len(words)-1]

	expanded := expandTilde(currentWord)

	var searchDir, searchPrefix string
	if currentWord == "" {
		searchDir = "."
		searchPrefix = ""
	} else if strings.HasSuffix(currentWord, "/") {
		searchDir = expanded
		searchPrefix = ""
	} else if currentWord == "~" {
		searchDir = expanded
		searchPrefix = ""
	} else {
		searchDir = filepath.Dir(expanded)
		searchPrefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return []string{}
	}

	var completions []string
	for _, entry := range entries {
		// 1. Filter by what the user has typed so far
		if !strings.HasPrefix(entry.Name(), searchPrefix) {
			continue
		}

		// 2. Bash-like hidden file filtering: hide dotfiles unless explicitly requested
		if !strings.HasPrefix(searchPrefix, ".") && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		var completedWord string

		// 3. Reconstruct the completion string based on the user's input prefix
		if currentWord == "" || strings.HasSuffix(currentWord, "/") {
			completedWord = currentWord + entry.Name()
		} else if currentWord == "~" {
			completedWord = "~/" + entry.Name()
		} else {
			dirPart := filepath.Dir(currentWord)
			if dirPart == "." && !strings.Contains(currentWord, "/") {
				completedWord = entry.Name()
			} else {
				if strings.HasSuffix(dirPart, "/") {
					completedWord = dirPart + entry.Name()
				} else {
					completedWord = dirPart + "/" + entry.Name()
				}
			}
		}

		// Add trailing slash for directories to encourage further navigation
		if entry.IsDir() {
			completedWord += "/"
		}

		// 4. Re-stitch the string together if the user is typing multiple files
		if len(words) > 1 {
			prefix := strings.Join(words[:len(words)-1], " ")
			completions = append(completions, prefix+" "+completedWord)
		} else {
			completions = append(completions, completedWord)
		}
	}

	return completions
}
