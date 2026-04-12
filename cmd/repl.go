package cmd

import (
	"bufio"
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
var ResponsePipe *os.File

type InputEvent struct {
	Source  string // "terminal", "prompt_pipe", "control_pipe"
	Payload string
}

func setupPipes() {
	if Project.ProjectName == "" {
		return
	}
	home, _ := os.UserHomeDir()
	runDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run")
	os.MkdirAll(runDir, 0755)

	pipes := []string{
		filepath.Join(runDir, "prompt.pipe"),
		filepath.Join(runDir, "control.pipe"),
		filepath.Join(runDir, "response.pipe"),
	}

	for _, p := range pipes {
		if info, err := os.Stat(p); err == nil {
			if info.Mode()&os.ModeNamedPipe == 0 {
				os.Remove(p)
				syscall.Mkfifo(p, 0600)
			}
		} else {
			syscall.Mkfifo(p, 0600)
		}
	}

	// Open Response Pipe as O_RDWR so it never blocks/closes on the writer side when Emacs disconnects
	var err error
	ResponsePipe, err = os.OpenFile(pipes[2], os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("[Warning] Could not open response.pipe: %v\n", err)
	}
}

func listenPromptPipe(eventChan chan<- InputEvent) {
	home, _ := os.UserHomeDir()
	pipePath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run", "prompt.pipe")
	for {
		pFile, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		bytes, _ := io.ReadAll(pFile)
		pFile.Close()

		prompt := strings.TrimSpace(string(bytes))
		if prompt != "" {
			eventChan <- InputEvent{Source: "prompt_pipe", Payload: prompt}
		}
	}
}

func listenControlPipe(eventChan chan<- InputEvent) {
	home, _ := os.UserHomeDir()
	pipePath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "run", "control.pipe")
	for {
		cFile, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		scanner := bufio.NewScanner(cFile)
		for scanner.Scan() {
			cmdStr := strings.TrimSpace(scanner.Text())
			if cmdStr != "" {
				eventChan <- InputEvent{Source: "control_pipe", Payload: cmdStr}
			}
		}
		cFile.Close()
	}
}

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

	ctx := context.Background()

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		RunHeadless(ctx)
		return
	}

	var completer = readline.NewPrefixCompleter(
		readline.PcItem("quit"),
		readline.PcItem("exit"),
		readline.PcItem("clear"),
		readline.PcItem("list", readline.PcItem("projects"), readline.PcItem("models")),
		readline.PcItem("show", readline.PcItem("global"), readline.PcItem("project")),
		readline.PcItem("set",
			readline.PcItem("model"),
			readline.PcItem("source_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("chunks_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("ingest_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("export_date"),
			readline.PcItem("project_name"),
		),
		readline.PcItem("context-include", readline.PcItemDynamic(listFiles)),
		readline.PcItem("inc", readline.PcItemDynamic(listFiles)),
		readline.PcItem("git-add-repo", readline.PcItemDynamic(listFiles)),
		readline.PcItem("git-remote", readline.PcItem("push"), readline.PcItem("pull"), readline.PcItem("clone")),
		readline.PcItem("gr",
			readline.PcItem("push"),
			readline.PcItem("pull"),
			readline.PcItem("clone"),
			readline.PcItemDynamic(listFiles),
		),
		readline.PcItem("cc"),
		readline.PcItem("p"),
		readline.PcItem("ps"),
		readline.PcItem("cr"),
		readline.PcItem("cw", readline.PcItemDynamic(listFiles)),
		readline.PcItem("wf"),
		readline.PcItem("write-files"),
		readline.PcItem("gl"),
		readline.PcItem("gd"),
		readline.PcItem("gs"),
	)

	home, _ := os.UserHomeDir()
	historyPath := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "ggo_history")

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

	fmt.Printf("Initializing Gemini API... ")
	err = InitGemini(ctx)
	if err != nil {
		fmt.Printf("\n[Warning] AI disabled: %v\n", err)
	} else {
		fmt.Printf("[OK]\n")
	}

	// 1. Initialize IPC
	setupPipes()

	// 2. Create the Central Event Bus
	eventChan := make(chan InputEvent)

	// 3. Start Pipe Listeners
	go listenPromptPipe(eventChan)
	go listenControlPipe(eventChan)

	// 4. Start Terminal Listener
	go func() {
		for {
			line, err := l.Readline()
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					eventChan <- InputEvent{Source: "terminal", Payload: "exit"}
					return
				}
				continue
			} else if err == io.EOF {
				eventChan <- InputEvent{Source: "terminal", Payload: "exit"}
				return
			}
			eventChan <- InputEvent{Source: "terminal", Payload: line}
		}
	}()

	fmt.Printf("Welcome to ggo [%s]. Unified Hub Active. Type 'quit' to exit.\n", Project.ProjectName)

	// 5. The Central State Machine (Multiplexer)
	for event := range eventChan {
		if event.Payload == "quit" || event.Payload == "exit" {
			fmt.Println("Flushing ledger and exiting...")
			break
		}

		var stdout io.Writer = os.Stdout
		if l != nil {
			stdout = l.Stdout()
		}

		if event.Source == "prompt_pipe" {
			fmt.Fprintf(stdout, "\n\033[35m[Emacs Prompt]\033[0m %s\n", event.Payload)
			executeCommand(ctx, event.Payload, l, true)
		} else if event.Source == "control_pipe" {
			fmt.Fprintf(stdout, "\n\033[35m[Emacs Control]\033[0m %s\n", event.Payload)
			if strings.ToUpper(event.Payload) == "WF" {
				handleWF()
			} else {
				executeCommand(ctx, event.Payload, l, true)
			}
		} else {
			executeCommand(ctx, event.Payload, l, false)
		}
	}

	CleanupTimeMachine()
}

func RunHeadless(ctx context.Context) {
	fmt.Printf("[ggo] Running in headless script mode...\n")
	InitGemini(ctx)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fmt.Printf("\n> %s\n", line)
		if exit := executeCommand(ctx, line, nil, false); exit {
			break
		}
	}
}

func executeCommand(ctx context.Context, line string, l *readline.Instance, fromPipe bool) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	if strings.HasPrefix(line, "!") {
		fmt.Println("-> Shell execution stubbed.")
		return false
	}

	parts := strings.Fields(line)
	cmd := parts[0]

	var stdout io.Writer = os.Stdout
	if l != nil {
		stdout = l.Stdout()
	}

	switch cmd {
	case "quit", "exit":
		return true
	case "clear":
		fmt.Println("-> Stub: Clearing active chat context")
	case "list":
		handleList(ctx, parts)
	case "show":
		handleShow(parts)
	case "set":
		handleSet(parts)
	case "context-include", "inc":
		handleContextInclude(ctx, parts)
	case "git-add-repo":
		handleGitAddRepo(parts)
	case "gr", "git-remote":
		if len(parts) >= 2 && (parts[1] == "push" || parts[1] == "pull" || parts[1] == "clone") {
			handleGitRemote(parts)
		} else if cmd == "gr" {
			handleGitAddRepo(parts)
		} else {
			fmt.Println("Usage: git-remote <push|pull|clone> [project]")
		}
	case "cc":
		handleCC(parts)
	case "p":
		if l != nil {
			handleP(ctx, l)
		} else {
			fmt.Println("[!] Multiline 'p' disabled in headless mode.")
		}
	case "ps":
		if l != nil {
			handlePS(ctx, l)
		} else {
			fmt.Println("[!] Multiline 'ps' disabled in headless mode.")
		}
	case "cr":
		handleCR()
	case "cw":
		handleCW(parts)
	case "wf", "write-files":
		handleWF()
	case "gl":
		handleGL(parts)
	case "gd":
		handleGD(parts)
	case "gs":
		handleGS(parts)
	default:
		sendAndRender(ctx, line, fromPipe, stdout)
	}
	return false
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
				fmt.Println("[Error] Invalid model number.")
				return
			}
		}
	}

	if strings.HasSuffix(field, "_dir") || strings.HasSuffix(field, "path") {
		value = expandTilde(value)
	}

	isGlobal := GlobalViper.IsSet(field) || field == "export_date" || field == "source_dir" || field == "chunks_dir" || field == "ingest_dir"
	isProject := ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd" || field == "model_name"

	ExecuteTransaction("set_"+field, func(anchor string, now time.Time) error {
		if isGlobal {
			GlobalViper.Set(field, value)
			GlobalViper.WriteConfig()
			InitConfig()
		} else if isProject {
			ProjectViper.Set(field, value)
			home, _ := os.UserHomeDir()
			ProjectViper.WriteConfigAs(filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "project.yaml"))
			InitConfig()
			if field == "model_name" {
				UpdateChatModel()
			}
		}
		return nil
	})
	fmt.Printf("[✓] Updated: %s = %s\n", field, value)
}

func expandTilde(path string) string {
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

func listFiles(line string) []string {
	expanded := expandTilde(line)
	search := expanded + "*"
	if expanded == "" {
		search = "*"
	}

	matches, err := filepath.Glob(search)
	if err != nil {
		return []string{}
	}

	var completions []string
	for _, match := range matches {
		displayPath := match
		if strings.HasPrefix(line, "~") {
			home, _ := os.UserHomeDir()
			displayPath = strings.Replace(match, home, "~", 1)
		}

		if stat, err := os.Stat(match); err == nil && stat.IsDir() {
			completions = append(completions, displayPath+"/")
		} else {
			completions = append(completions, displayPath)
		}
	}
	return completions
}
