package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func handleInc(ctx context.Context, parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: inc <path1> [path2...]")
		return
	}

	if chatSession == nil {
		fmt.Println("[Error] Cannot ingest files: Gemini API is not initialized.")
		return
	}

	paths := parts[1:]
	var addedFiles []string
	var ingestionBuilder strings.Builder

	// The Master Timestamp for this ingestion turn
	timestamp := time.Now().Format("20060102T150405")
	home, _ := os.UserHomeDir()
	centralRepoDir := filepath.Join(home, ".ggo", "ggoai-"+Project.ProjectName)
	centralIncludesDir := filepath.Join(centralRepoDir, "includes")

	fmt.Printf("Scanning and syncing paths...\n")

	for _, targetPath := range paths {
		absTargetPath, _ := filepath.Abs(targetPath)
		
		err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			// Skip hidden directories like .git and .ggo
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.IsDir() {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(contentBytes)

			// Simple binary skip
			if strings.Contains(content, "\x00") {
				return nil
			}

			addedFiles = append(addedFiles, path)
			ingestionBuilder.WriteString(fmt.Sprintf("--- START FILE: %s ---\n%s\n--- END FILE: %s ---\n\n", path, content, path))

			// (iii) Copy to the central LFS repo's includes directory
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

		// (ii) Check for a .git directory to initialize/commit a shadow repo
		if hasGitDir(absTargetPath) {
			safeName := strings.ReplaceAll(absTargetPath, "/", "_")
			shadowGitDir := filepath.Join(centralRepoDir, "shadow-repos", safeName+".git")
			
			if _, err := os.Stat(shadowGitDir); os.IsNotExist(err) {
				fmt.Printf("Initializing shadow repo for %s...\n", targetPath)
				os.MkdirAll(shadowGitDir, 0755)
				exec.Command("git", "--git-dir="+shadowGitDir, "init", "--bare").Run()
			}

			err = SnapshotGit(shadowGitDir, absTargetPath, "inc timestamp: "+timestamp, timestamp)
			if err != nil {
				fmt.Printf("[Warning] Shadow snapshot failed for %s: %v\n", targetPath, err)
			}
		}
	}

	if len(addedFiles) == 0 {
		fmt.Println("[!] No readable text files found.")
		return
	}

	// Update persistent project.yaml
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

	// Commit & Tag the central LFS repo
	err := SnapshotGit(filepath.Join(centralRepoDir, ".git"), centralRepoDir, "inc timestamp: "+timestamp, timestamp)
	if err != nil {
		fmt.Printf("[Warning] Central LFS snapshot failed: %v\n", err)
	} else {
		fmt.Printf("[✓] State snapshotted and tagged: %s\n", timestamp)
	}

	// Finally, send the payload to Gemini
	fmt.Printf("Ingesting %d files into AI context...\033[90m (Thinking...)\033[0m\n", len(addedFiles))
	prompt := "Please silently ingest the following files into your context for our project. Acknowledge by replying ONLY with 'Files ingested successfully.' Do not summarize or repeat the contents.\n\n" + ingestionBuilder.String()
	
	resp, err := SendPrompt(ctx, prompt)
	if err != nil {
		fmt.Printf("[API Error during ingestion] %v\n", err)
	} else {
		lastAPIResponse = resp
		fmt.Printf("[✓] %s\n", strings.TrimSpace(resp))
	}
}

// copyFile is a simple utility to copy a file from src to dst
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

// hasGitDir searches upward from the given path to see if it belongs to a git repo
func hasGitDir(path string) bool {
	dir := path
	for {
		if stat, err := os.Stat(filepath.Join(dir, ".git")); err == nil && stat.IsDir() {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}
