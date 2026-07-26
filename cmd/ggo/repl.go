package ggo

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
	"github.com/peterjbraam/ggo/internal/core"
)

var lastAPIResponse string
var ResponsePipe *os.File

type InputEvent struct {
	Source  string
	Payload string
}

func setupPipes() {
	if core.ProjectCfg.ProjectName == "" {
		return
	}
	runDir := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "run")
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

	var err error
	ResponsePipe, err = os.OpenFile(pipes[2], os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("[Warning] Could not open response.pipe: %v\n", err)
	}
}

func listenPromptPipe(eventChan chan<- InputEvent) {
	pipePath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "run", "prompt.pipe")
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
	pipePath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "run", "control.pipe")
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
	core.InitConfig(core.ProjectCfg.ProjectName)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if core.Runtime.IsThinking && sig == os.Interrupt {
				continue
			}
			fmt.Println("\n[!] Caught termination signal. Cleaning up...")
			core.CleanupTimeMachine()
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
		readline.PcItem("show",
			readline.PcItem("global"),
			readline.PcItem("project"),
			readline.PcItem("costs"),
			readline.PcItem("params"),
			readline.PcItem("roles"),
		),
		readline.PcItem("add", readline.PcItem("role")),
		readline.PcItem("set",
			readline.PcItem("model"),
			readline.PcItem("role", readline.PcItem("developer"), readline.PcItem("architect"), readline.PcItem("challenger"), readline.PcItem("writer"), readline.PcItem("math")),
			readline.PcItem("temperature"),
			readline.PcItem("top_p"),
			readline.PcItem("safety_level"),
			readline.PcItem("diff_cmd"),
			readline.PcItem("records_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("chunks_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("ingest_dir", readline.PcItemDynamic(listFiles)),
			readline.PcItem("export_date"),
			readline.PcItem("project_name"),
		),
		readline.PcItem("context",
			readline.PcItem("add", readline.PcItemDynamic(listFiles)),
			readline.PcItem("list", readline.PcItem("-l")),
			readline.PcItem("delete"),
		),
		readline.PcItem("file",
			readline.PcItem("list"),
			readline.PcItem("write"),
			readline.PcItem("merge"),
			readline.PcItem("clear"),
		),
		readline.PcItem("inc", readline.PcItemDynamic(listFiles)),
		readline.PcItem("incl", readline.PcItem("-l")),
		readline.PcItem("incd"),
		readline.PcItem("ca", readline.PcItemDynamic(listFiles)),
		readline.PcItem("cl", readline.PcItem("-l")),
		readline.PcItem("cs", readline.PcItem("-l")),
		readline.PcItem("cd"),
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
		readline.PcItem("fl"),
		readline.PcItem("fw"),
		readline.PcItem("fm"),
		readline.PcItem("fc"),
		readline.PcItem("wf"),
		readline.PcItem("write-files"),
		readline.PcItem("gl"),
		readline.PcItem("git-ledger"),
		readline.PcItem("gd"),
		readline.PcItem("git-diff"),
		readline.PcItem("gs"),
	)

	historyPath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "ggo_history")

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
	err = core.InitGemini(ctx)
	if err != nil {
		fmt.Printf("\n[Warning] AI disabled: %v\n", err)
	} else {
		fmt.Printf("[OK]\n")
	}

	fmt.Println("Starting Web UI...")
	webUI := NewWebServer()
	go webUI.Start("8080")

	setupPipes()
	reviveLastResponse(core.GetActiveChat())

	eventChan := make(chan InputEvent)
	termDoneChan := make(chan bool)

	go listenPromptPipe(eventChan)
	go listenControlPipe(eventChan)

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

			<-termDoneChan
		}
	}()

	fmt.Printf("Welcome to ggo [%s]. Unified Hub Active. Type 'quit' to exit.\n", core.ProjectCfg.ProjectName)

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

			payloadParts := strings.Fields(event.Payload)
			payloadCmd := strings.ToLower(payloadParts[0])
			if payloadCmd == "wf" || payloadCmd == "fw" {
				handleFileRouter("fw", payloadParts)
			} else {
				executeCommand(ctx, event.Payload, l, true)
			}
		} else {
			executeCommand(ctx, event.Payload, l, false)
		}

		if event.Source == "terminal" {
			termDoneChan <- true
		}
	}

	core.CleanupTimeMachine()
}

func RunHeadless(ctx context.Context) {
	fmt.Printf("[ggo] Running in headless script mode...\n")
	core.InitGemini(ctx)

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
	case "add":
		if len(parts) > 1 && parts[1] == "role" {
			handleAddRole(parts)
		} else {
			fmt.Println("Usage: add role <name> <description_string_or_filepath.md>")
		}
	case "show":
		if len(parts) > 1 && parts[1] == "costs" {
			handleShowCosts(parts)
		} else if len(parts) > 1 && parts[1] == "params" {
			handleShowParams()
		} else if len(parts) > 1 && parts[1] == "roles" {
			handleShowRoles()
		} else if len(parts) > 1 && parts[1] == "projects" {
			fmt.Println("\n--- Tracked ggo Projects ---")
			if len(core.GlobalSt.Projects) == 0 {
				fmt.Println("No projects tracked yet.")
			} else {
				for name, path := range core.GlobalSt.Projects {
					fmt.Printf("- %-15s [%s]\n", name, path)
				}
			}
			fmt.Println()
		} else if len(parts) > 1 && parts[1] == "models" {
			core.FetchAndListModels(ctx)
		} else {
			handleShow(parts)
		}
	case "set":
		handleSet(parts)
	case "context", "inc", "incl", "incd", "ca", "cl", "cs", "cd":
		handleContextRouter(ctx, cmd, parts)
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
	case "file", "fl", "fw", "fm", "fc", "wf", "write-files":
		handleFileRouter(cmd, parts)
	case "gl", "git-ledger":
		handleGL(parts)
	case "gd", "git-diff":
		handleGD(parts)
	case "gs":
		handleGS(parts)
	case "prompt":
		if len(parts) < 2 {
			fmt.Println("[Error] Usage: prompt <your question here>")
			return false
		}
		sendAndRender(ctx, strings.Join(parts[1:], " "), fromPipe, stdout)
	default:
		if fromPipe {
			sendAndRender(ctx, line, fromPipe, stdout)
		} else {
			fmt.Printf("\033[31m[!] Unknown command: '%s'\033[0m\n", cmd)
			fmt.Println("    To send a prompt to the AI, prefix it with 'prompt' (e.g., 'prompt how do I fix this?')")
			fmt.Println("    Or use the 'p' command to enter multi-line chat mode.")
		}
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
		if len(core.GlobalSt.Projects) == 0 {
			fmt.Println("No projects tracked yet.")
		} else {
			for name, path := range core.GlobalSt.Projects {
				fmt.Printf("- %-15s [%s]\n", name, path)
			}
		}
		fmt.Println()
	} else if target == "models" {
		core.FetchAndListModels(ctx)
	} else {
		fmt.Println("Unknown list target. Use 'projects' or 'models'.")
	}
}

func handleShow(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: show <global|project|costs|params|roles>")
		return
	}
	target := parts[1]
	if target == "global" {
		fmt.Println("\n--- Global Config ---")
		for k, v := range core.GlobalViper.AllSettings() {
			fmt.Printf("%s: %v\n", k, v)
		}
		fmt.Println()
	} else if target == "project" {
		fmt.Println("\n--- Project Config ---")
		for k, v := range core.ProjectViper.AllSettings() {
			fmt.Printf("%s: %v\n", k, v)
		}
		fmt.Println()
	} else {
		fmt.Println("Unknown show target. Use 'global', 'project', 'costs', 'params', or 'roles'.")
	}
}

func handleShowParams() {
	fmt.Println("\n--- Model Tuning Parameters ---")

	temp := 0.2
	if core.ProjectViper.IsSet("temperature") {
		temp = core.ProjectViper.GetFloat64("temperature")
	}
	fmt.Printf("%-15s: %.2f  (0.0 = Strict/Coding, 2.0 = Brainstorming)\n", "Temperature", temp)

	topP := 0.95
	if core.ProjectViper.IsSet("top_p") {
		topP = core.ProjectViper.GetFloat64("top_p")
	}
	fmt.Printf("%-15s: %.2f  (0.0 to 1.0, restricts vocabulary choices)\n", "TopP", topP)

	safety := "default"
	if core.ProjectViper.IsSet("safety_level") {
		safety = core.ProjectViper.GetString("safety_level")
	}
	fmt.Printf("%-15s: %s  (none, low, default, high)\n", "Safety Level", safety)

	fmt.Println("\nUse 'set <parameter> <value>' to change these.")
	fmt.Println("-------------------------------")
}

func handleAddRole(parts []string) {
	if len(parts) < 4 || parts[1] != "role" {
		fmt.Println("Usage: add role <name> <description_string_or_filepath.md>")
		return
	}

	roleName := parts[2]
	descOrFile := strings.Join(parts[3:], " ")
	expanded := expandTilde(descOrFile)

	var roleContent []byte
	if info, err := os.Stat(expanded); err == nil && !info.IsDir() {
		roleContent, err = os.ReadFile(expanded)
		if err != nil {
			fmt.Printf("[Error] Could not read file %s: %v\n", expanded, err)
			return
		}
	} else {
		roleContent = []byte(descOrFile)
	}

	projVault := core.GetProjectVaultDir(core.ProjectCfg.ProjectName)
	rolePath := filepath.Join(projVault, fmt.Sprintf("role-%s.md", roleName))

	core.ExecuteTransaction("add_role_"+roleName, func(anchor string, now time.Time) error {
		os.MkdirAll(projVault, 0755)
		err := os.WriteFile(rolePath, roleContent, 0644)
		if err != nil {
			return err
		}

		core.ProjectViper.Set("role", roleName)
		core.ProjectViper.WriteConfigAs(filepath.Join(projVault, "project.yaml"))
		core.InitConfig(core.ProjectCfg.ProjectName)
		core.UpdateChatModel()
		return nil
	})

	fmt.Printf("[✓] Created and activated new role: %s\n", roleName)
}

func handleShowRoles() {
	fmt.Println("\n--- Available Personas ---")

	activeRole := "developer"
	if core.ProjectViper.IsSet("role") {
		activeRole = core.ProjectViper.GetString("role")
	}

	projVault := core.GetProjectVaultDir(core.ProjectCfg.ProjectName)

	rolesMap := make(map[string]string)
	for k, v := range core.DefaultRoles {
		rolesMap[k] = v
	}

	matches, _ := filepath.Glob(filepath.Join(projVault, "role-*.md"))
	for _, match := range matches {
		base := filepath.Base(match)
		name := strings.TrimPrefix(base, "role-")
		name = strings.TrimSuffix(name, ".md")

		content, err := os.ReadFile(match)
		if err == nil {
			rolesMap[name] = strings.TrimSpace(string(content))
		}
	}

	for name, desc := range rolesMap {
		prefix := "  "
		if name == activeRole {
			prefix = "\033[32m*\033[0m "
		}

		truncDesc := strings.ReplaceAll(desc, "\n", " ")
		if len(truncDesc) > 80 {
			truncDesc = truncDesc[:77] + "..."
		}
		fmt.Printf("%s%-15s: %s\n", prefix, name, truncDesc)
	}

	fmt.Println("\nUse 'set role <name>' to switch personas.")
	fmt.Println("Use 'add role <name> <desc_or_file.md>' to dynamically create a new one.")
	fmt.Println("--------------------------")
}

func handleShowCosts(parts []string) {
	logPath := filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "cost.csv")

	data, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Println("[Info] No cost data recorded yet for this project.")
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= 1 {
		fmt.Println("[Info] No cost data recorded yet.")
		return
	}

	filterDate := time.Now().UTC().Format("2006-01") + "-01T00:00:00Z"
	title := "Costs (Since start of month)"

	if len(parts) > 2 {
		if strings.ToLower(parts[2]) == "all" {
			filterDate = ""
			title = "Costs (All Time)"
		} else {
			filterDate = parts[2]
			if len(filterDate) == 10 {
				filterDate += "T00:00:00Z"
			}
			title = fmt.Sprintf("Costs (Since %s)", parts[2])
		}
	}

	var totalIn, totalOut, totalTokens int
	var totalCost float64
	var printCount int

	fmt.Printf("\n--- Project Cost Summary: %s ---\n", title)
	fmt.Printf("%-25s %-12s %-12s %-12s %-15s\n", "Date (UTC)", "Input", "Output", "Total", "Est. Cost (USD)")
	fmt.Println(strings.Repeat("-", 80))

	for i := 1; i < len(lines); i++ {
		cols := strings.Split(lines[i], ",")
		if len(cols) < 6 {
			continue
		}

		if filterDate != "" && cols[0] < filterDate {
			continue
		}

		inTok, _ := strconv.Atoi(cols[2])
		outTok, _ := strconv.Atoi(cols[3])
		totTok, _ := strconv.Atoi(cols[4])
		costStr := strings.TrimPrefix(cols[5], "$")
		cost, _ := strconv.ParseFloat(costStr, 64)

		totalIn += inTok
		totalOut += outTok
		totalTokens += totTok
		totalCost += cost

		if len(lines)-i <= 5 {
			fmt.Printf("%-25s %-12d %-12d %-12d $%.6f\n", cols[0], inTok, outTok, totTok, cost)
			printCount++
		}
	}

	if printCount == 0 {
		fmt.Println("  (No transactions found in this period)")
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("\033[32m%-25s %-12d %-12d %-12d $%.6f\033[0m\n\n", "TOTAL", totalIn, totalOut, totalTokens, totalCost)

	if filterDate != "" {
		fmt.Println("(Hint: Run 'show costs all' to view all-time ledger)")
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
			if idx > 0 && idx <= len(core.View.ModelList) {
				value = core.View.ModelList[idx-1]
			} else {
				fmt.Println("[Error] Invalid model number.")
				return
			}
		}
	}

	if strings.HasSuffix(field, "_dir") || strings.HasSuffix(field, "path") {
		value = expandTilde(value)
	}

	var finalValue interface{} = value

	if field == "temperature" || field == "top_p" {
		if vFloat, err := strconv.ParseFloat(value, 64); err == nil {
			finalValue = vFloat
		} else {
			fmt.Printf("[Error] %s must be a numeric value.\n", field)
			return
		}
	}

	if field == "export_date" {
		core.GlobalSt.ExportDate = finalValue.(string)
		core.SaveGlobalState()
		fmt.Printf("[✓] Updated state: export_date = %v\n", finalValue)
		return
	}

	if field == "project_name" {
		projName := finalValue.(string)
		cwd, _ := os.Getwd()
		if core.GlobalSt.Projects == nil {
			core.GlobalSt.Projects = make(map[string]string)
		}
		core.GlobalSt.Projects[projName] = cwd
		core.GlobalSt.LastProject = projName
		core.SaveGlobalState()

		fmt.Printf("\033[32m[✓] Registered new workspace:\033[0m %s -> %s\n", projName, cwd)
		core.InitConfig(core.ProjectCfg.ProjectName)
		return
	}

	isGlobalConfig := core.GlobalViper.IsSet(field) || field == "records_dir" || field == "chunks_dir" || field == "ingest_dir" || field == "api_key_file"
	isProject := core.ProjectViper.IsSet(field) || field == "project_name" || field == "shell_cmd" || field == "diff_cmd" || field == "model_name" || field == "temperature" || field == "top_p" || field == "safety_level" || field == "role"

	core.ExecuteTransaction("set_"+field, func(anchor string, now time.Time) error {
		if isGlobalConfig {
			core.GlobalViper.Set(field, finalValue)
			core.GlobalViper.WriteConfigAs(filepath.Join(core.GetConfigDir(), "config.yaml"))
			core.InitConfig(core.ProjectCfg.ProjectName)
		} else if isProject {
			core.ProjectViper.Set(field, finalValue)
			core.ProjectViper.WriteConfigAs(filepath.Join(core.GetProjectVaultDir(core.ProjectCfg.ProjectName), "project.yaml"))
			core.InitConfig(core.ProjectCfg.ProjectName)

			if field == "model_name" || field == "temperature" || field == "top_p" || field == "safety_level" || field == "role" {
				core.UpdateChatModel()
			}
		}
		return nil
	})
	fmt.Printf("[✓] Updated config: %s = %v\n", field, finalValue)
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
