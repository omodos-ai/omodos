package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// findGitRoot walks up the directory tree until it finds a .git file or directory.
func findGitRoot(startPath string) string {
	dir, err := filepath.Abs(startPath)
	if err != nil { return "" }
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // Reached filesystem root
			return ""
		}
		dir = parent
	}
}

func handleInc(ctx context.Context, parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: inc <path1> [path2...]")
		return
	}

	paths := parts[1:]

	// --- Pre-flight Auto-Enrollment of Git Repositories ---
	// This runs perfectly even if you are offline!
	addedNewRepo := false
	for _, p := range paths {
		gitRoot := findGitRoot(p)
		if gitRoot != "" {
			exists := false
			for _, c := range Project.CodeDirs {
				if c == gitRoot {
					exists = true
					break
				}
			}
			if !exists {
				Project.CodeDirs = append(Project.CodeDirs, gitRoot)
				addedNewRepo = true
				fmt.Printf("\033[32m[Info] Auto-enrolling new Git repository:\033[0m %s\n", gitRoot)
			}
		}
	}

	// If we found new repos, save them to the central config and capture a baseline instantly.
	if addedNewRepo {
		home, _ := os.UserHomeDir()
		ProjectViper.Set("code_dirs", Project.CodeDirs)
		ProjectViper.WriteConfigAs(filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName, "project.yaml"))
		
		anchor := time.Now().UTC().Format("20060102T150405Z")
		SyncAllShadows(anchor, "init", "init: auto-enrolled repository baseline")
	}

	// --- NOW check for the AI Connection before ingesting ---
	if chatSession == nil {
		fmt.Println("[Error] Cannot ingest files: Gemini API is not initialized.")
		return
	}

	// --- Standard Transaction / Ingestion Pipeline ---
	var addedFiles []string
	var ingestionBuilder strings.Builder

	ExecuteTransaction("ingest_files", func(anchor string, now time.Time) error {
		home, _ := os.UserHomeDir()
		centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
		centralIncludesDir := filepath.Join(centralRepoDir, "includes")

		fmt.Printf("Scanning and syncing paths...\n")

		for _, targetPath := range paths {
			err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil { return err }
				
				if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
					return filepath.SkipDir
				}
				if info.IsDir() { return nil }

				contentBytes, err := os.ReadFile(path)
				if err != nil { return nil }
				content := string(contentBytes)

				if strings.Contains(content, "\x00") { return nil }

				addedFiles = append(addedFiles, path)
				ingestionBuilder.WriteString(fmt.Sprintf("--- START FILE: %s ---\n%s\n--- END FILE: %s ---\n\n", path, content, path))

				absPath, _ := filepath.Abs(path)
				cwd, _ := os.Getwd()
				relPath, err := filepath.Rel(cwd, absPath)
				if err == nil && !strings.HasPrefix(relPath, "..") {
					destPath := filepath.Join(centralIncludesDir, relPath)
					os.MkdirAll(filepath.Dir(destPath), 0755)
					copyFile(path, destPath)
				}
				return nil
			})

			if err != nil {
				fmt.Printf("[Error] Could not process path '%s': %v\n", targetPath, err)
				continue
			}
		}

		if len(addedFiles) == 0 {
			fmt.Println("[!] No readable text files found.")
			return nil
		}

		existing := make(map[string]bool)
		for _, item := range Project.IncludedData {
			existing[item] = true
		}
		for _, p := range paths {
			if !existing[p] {
				Project.IncludedData = append(Project.IncludedData, p)
				existing[p] = true
			}
		}
		ProjectViper.Set("included_data", Project.IncludedData)
		ProjectViper.WriteConfigAs(filepath.Join(centralRepoDir, "project.yaml"))

		fmt.Printf("Ingesting %d files into AI context...\033[90m (Thinking...)\033[0m\n", len(addedFiles))
		prompt := "Please silently ingest the following files into your context for our project. Acknowledge by replying ONLY with 'Files ingested successfully.' Do not summarize or repeat the contents.\n\n" + ingestionBuilder.String()
		
		resp, err := SendPrompt(ctx, prompt)
		if err != nil {
			fmt.Printf("[API Error during ingestion] %v\n", err)
			return err
		}
		lastAPIResponse = resp
		fmt.Printf("[✓] %s\n", strings.TrimSpace(resp))
		return nil
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
