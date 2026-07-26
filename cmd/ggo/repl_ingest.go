package ggo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
)

func printContextHelp() {
	fmt.Println("\n--- Context Management Subsystem ---")
	fmt.Println("Manage the Just-In-Time (JIT) file baseline monitored by the AI.")
	fmt.Println("\nCommands:")
	fmt.Println("  context add <path...>   (Aliases: inc, ca)   - Enrolls files/directories into the AI context.")
	fmt.Println("  context list [-l]       (Aliases: incl, cl)  - Displays footprint. Use -l to list all files.")
	fmt.Println("  context delete <target> (Aliases: incd, cd)  - Removes a monitored root by its index or exact path.")
	fmt.Println("\nExamples:")
	fmt.Println("  inc src/main.go")
	fmt.Println("  cl -l")
	fmt.Println("  cd 0")
	fmt.Println()
}

// handleContextRouter intercepts both the long 'context' command and its short aliases
func handleContextRouter(ctx context.Context, cmd string, parts []string) {
	// 1. Intercept help requests immediately
	for _, p := range parts {
		if p == "help" || p == "-h" || p == "--help" {
			printContextHelp()
			return
		}
	}

	// 2. Route the command
	if cmd == "inc" || cmd == "ca" || (cmd == "context" && len(parts) > 1 && parts[1] == "add") {
		paths := parts[1:]
		if cmd == "context" {
			paths = parts[2:]
		}
		if len(paths) == 0 {
			fmt.Println("[Error] Missing path. Usage: context add <path...>")
			return
		}
		handleContextAdd(ctx, paths)

	} else if cmd == "incl" || cmd == "cl" || (cmd == "context" && len(parts) > 1 && parts[1] == "list") {
		handleContextList(parts)

	} else if cmd == "incd" || cmd == "cd" || (cmd == "context" && len(parts) > 1 && parts[1] == "delete") {
		args := parts[1:]
		if cmd == "context" {
			args = parts[2:]
		}
		if len(args) == 0 {
			fmt.Println("[Error] Missing target. Usage: context delete <index|path>")
			return
		}
		handleContextDelete(args)

	} else {
		printContextHelp()
	}
}

func handleContextAdd(ctx context.Context, paths []string) {
	if core.ChatSession == nil {
		fmt.Println("[Error] Cannot ingest files: Gemini API is not initialized.")
		return
	}

	gitRoots := make(map[string]bool)
	for _, p := range paths {
		root := findGitRoot(p)
		if root != "" && !gitRoots[root] {
			gitRoots[root] = true
			gitignorePath := filepath.Join(root, ".gitignore")
			if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
				fmt.Printf("\033[33m[Warning]\033[0m No .gitignore found in repository root (%s).\n", root)
			}
		}
	}

	var addedFiles []string
	var ingestionBuilder strings.Builder
	var totalBytes int64

	centralRepoDir := core.GetProjectVaultDir(core.ProjectCfg.ProjectName)
	centralIncludesDir := filepath.Join(centralRepoDir, "includes")

	fmt.Printf("Scanning paths for AI Context...\n")

	for _, targetPath := range paths {
		root := findGitRoot(targetPath)

		err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}

			if root != "" {
				checkIgnored := exec.Command("git", "check-ignore", "-q", path)
				checkIgnored.Dir = root
				if err := checkIgnored.Run(); err == nil {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				if !info.IsDir() {
					checkTracked := exec.Command("git", "ls-files", "--error-unmatch", path)
					checkTracked.Dir = root
					if err := checkTracked.Run(); err != nil {
						return nil
					}
				}
			}

			if info.IsDir() {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(contentBytes)

			if strings.Contains(content, "\x00") {
				return nil
			}

			addedFiles = append(addedFiles, path)
			totalBytes += int64(len(contentBytes))
			ingestionBuilder.WriteString(fmt.Sprintf("--- START FILE: %s ---\n%s\n--- END FILE: %s ---\n\n", path, content, path))
			return nil
		})

		if err != nil {
			fmt.Printf("[Error] Could not process path '%s': %v\n", targetPath, err)
			continue
		}
	}

	if len(addedFiles) == 0 {
		fmt.Println("[!] No readable text files found.")
		return
	}

	// Safety Valve: Token Bloat Warning
	estTokens := totalBytes / 4
	if estTokens > 50000 {
		sendCost := float64(estTokens) * (1.25 / 1000000.0)
		cacheCost := float64(estTokens) * (0.30 / 1000000.0)

		fmt.Printf("\n\033[33m[Warning] Massive Context Payload Detected\033[0m\n")
		fmt.Printf("Files to add : %d\n", len(addedFiles))
		fmt.Printf("Payload size : %.2f MB\n", float64(totalBytes)/(1024*1024))
		fmt.Printf("Est. Tokens  : ~%d\n", estTokens)
		fmt.Printf("Est. Cost    : $%.4f to send, $%.4f/hr to cache\n", sendCost, cacheCost)
		fmt.Printf("\nAre you sure you want to ingest this? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("Ingestion aborted.")
			return
		}
	}

	core.ExecuteTransaction("context_add", func(anchor string, now time.Time) error {
		for _, path := range addedFiles {
			absPath, _ := filepath.Abs(path)
			cwd, _ := os.Getwd()
			relPath, err := filepath.Rel(cwd, absPath)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				destPath := filepath.Join(centralIncludesDir, relPath)
				os.MkdirAll(filepath.Dir(destPath), 0755)
				copyFile(path, destPath)
			}
		}

		existing := make(map[string]bool)
		for _, item := range core.ProjectSt.IncludedData {
			existing[item] = true
		}
		for _, p := range paths {
			if !existing[p] {
				core.ProjectSt.IncludedData = append(core.ProjectSt.IncludedData, p)
				existing[p] = true
			}
		}
		core.ProjectViper.Set("included_data", core.ProjectSt.IncludedData)
		core.ProjectViper.WriteConfigAs(filepath.Join(centralRepoDir, "project.yaml"))

		fmt.Printf("Ingesting %d files into AI context...\033[90m (Thinking...)\033[0m\n", len(addedFiles))
		prompt := "Please silently ingest the following files into your context for our project. Acknowledge by replying ONLY with 'Files ingested successfully.' Do not summarize or repeat the contents.\n\n" + ingestionBuilder.String()

		resp, err := core.SendPrompt(ctx, prompt)
		if err != nil {
			fmt.Printf("[API Error during ingestion] %v\n", err)
			return err
		}

		lastAPIResponse = resp
		fmt.Printf("[✓] %s\n", strings.TrimSpace(resp))
		return nil
	})
}

func handleContextList(parts []string) {
	if len(core.ProjectSt.IncludedData) == 0 {
		fmt.Println("\n[Info] No roots currently enrolled in AI context.")
		return
	}

	showList := false
	for _, p := range parts {
		if p == "-l" || p == "--list" {
			showList = true
			break
		}
	}

	var totalFiles int
	var totalBytes int64
	var fileList []string

	cwd, _ := os.Getwd()

	fmt.Println("\n--- Enrolled Context Roots ---")
	for i, rootPath := range core.ProjectSt.IncludedData {
		fmt.Printf("[%d] %s\n", i, rootPath)
		root := findGitRoot(rootPath)

		filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if root != "" {
				checkIgnored := exec.Command("git", "check-ignore", "-q", path)
				checkIgnored.Dir = root
				if checkIgnored.Run() == nil {
					return nil
				}
				checkTracked := exec.Command("git", "ls-files", "--error-unmatch", path)
				checkTracked.Dir = root
				if checkTracked.Run() != nil {
					return nil
				}
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(contentBytes), "\x00") {
				return nil
			}

			totalFiles++
			totalBytes += int64(len(contentBytes))

			relPath, err := filepath.Rel(cwd, path)
			if err == nil {
				fileList = append(fileList, relPath)
			} else {
				fileList = append(fileList, path)
			}
			return nil
		})
	}

	estTokens := totalBytes / 4

	fmt.Println("\n--- Resolved Footprint ---")
	fmt.Printf("Total Files : %d\n", totalFiles)
	fmt.Printf("Total Size  : %.2f MB\n", float64(totalBytes)/(1024*1024))
	fmt.Printf("Est. Tokens : ~%d\n", estTokens)

	if showList {
		fmt.Println("\n--- Tracked File Listing ---")
		for _, f := range fileList {
			fmt.Printf("- %s\n", f)
		}
	} else {
		fmt.Println("\n(Hint: Run 'cl -l' or 'context list -l' to view individual files)")
	}
	fmt.Println()
}

func handleContextDelete(args []string) {
	if len(core.ProjectSt.IncludedData) == 0 {
		fmt.Println("[Info] No context roots to delete.")
		return
	}

	target := args[0]
	var newRoots []string
	deleted := false

	// Try index deletion first
	if idx, err := strconv.Atoi(target); err == nil && idx >= 0 && idx < len(core.ProjectSt.IncludedData) {
		for i, p := range core.ProjectSt.IncludedData {
			if i == idx {
				fmt.Printf("[✓] Removed context root: %s\n", p)
				deleted = true
			} else {
				newRoots = append(newRoots, p)
			}
		}
	} else {
		// Fallback to exact path match deletion
		for _, p := range core.ProjectSt.IncludedData {
			if p == target {
				fmt.Printf("[✓] Removed context root: %s\n", p)
				deleted = true
			} else {
				newRoots = append(newRoots, p)
			}
		}
	}

	if !deleted {
		fmt.Printf("[Error] Context root '%s' not found. Use 'cl' to view valid indices and paths.\n", target)
		return
	}

	core.ProjectSt.IncludedData = newRoots
	core.ProjectViper.Set("included_data", core.ProjectSt.IncludedData)
	centralRepoDir := core.GetProjectVaultDir(core.ProjectCfg.ProjectName)
	core.ProjectViper.WriteConfigAs(filepath.Join(centralRepoDir, "project.yaml"))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
