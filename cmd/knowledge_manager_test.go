package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func TestMassiveRealWorldIngest(t *testing.T) {
	// 1. Unambiguous Physical Pathing
	_, thisFile, _, _ := runtime.Caller(0)
	cmdDir := filepath.Dir(thisFile)
	repoRoot := filepath.Dir(cmdDir)
	swDir := filepath.Dir(repoRoot)

	// Define the permanent sandbox paths
	baseTestDir := filepath.Join(swDir, "ggo-tests")
	downloadsDir := filepath.Join(baseTestDir, "downloads")
	ingestDir := filepath.Join(baseTestDir, "ingest")
	sourceDir := filepath.Join(baseTestDir, "sources")
	chunksDir := filepath.Join(baseTestDir, "chunks")

	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skipf("Skipping stress test: downloads directory not found at %s", downloadsDir)
	}

	// 2. Discover the zip files
	var geminiZip, chatgptZip string
	filepath.Walk(downloadsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.Contains(name, "takeout") && strings.HasSuffix(name, ".zip") {
			geminiZip = path
		} else if strings.HasSuffix(name, ".zip") && !strings.Contains(name, "takeout") {
			chatgptZip = path
		}
		return nil
	})

	if geminiZip == "" || chatgptZip == "" {
		t.Skip("Skipping stress test: could not find both Takeout and ChatGPT zips in " + downloadsDir)
	}

	// 3. Ephemeral Setup (Only for the binary and config to avoid littering)
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "ggo")
	mainPath := filepath.Join(repoRoot, "main.go")

	t.Log("Compiling ggo binary for isolated CLI test...")
	buildCmd := exec.Command("go", "build", "-o", binPath, mainPath)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile ggo binary: %v\nOutput: %s", err, string(out))
	}

	// 4. Native Config Initialization (Pointing to the PERMANENT directories)
	configYaml := fmt.Sprintf("ingest_dir: %s\nsource_dir: %s\nchunks_dir: %s\nexport_date: 2026-04-16\n",
		ingestDir, sourceDir, chunksDir)
	os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configYaml), 0644)

	// 5. Streamlined Movie Script
	shellScript := fmt.Sprintf(`
set -euo pipefail

# Point GGO to our ephemeral config, which routes data to the permanent sandbox
export GGO_CONFIG_DIR="%s"
GGO="%s"

echo "--- Extracting Archives (7zz) ---"
"$GGO" unzip gemini "%s" 2026-04-16 -f
"$GGO" unzip chatgpt "%s" 2026-04-16 -f

echo "--- Firing the Knowledge Manager ---"
"$GGO" chunks --conceptual
"$GGO" chunks
`, tempDir, binPath, geminiZip, chatgptZip)

	t.Log("Executing CLI workflow script via mvdan/sh...")
	parser := syntax.NewParser()
	prog, err := parser.Parse(strings.NewReader(shellScript), "e2e-workflow.sh")
	if err != nil {
		t.Fatalf("Failed to parse shell script: %v", err)
	}

	runner, err := interp.New(interp.StdIO(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		t.Fatalf("Failed to initialize shell interpreter: %v", err)
	}

	err = runner.Run(context.Background(), prog)
	if err != nil {
		t.Fatalf("CLI workflow script failed: %v", err)
	}

	// 6. Verification 1: Volume Check
	var jsonCount int
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonCount++
		}
		return nil
	})

	t.Logf("Extracted %d total interactions to strictly typed JSON.", jsonCount)
	if jsonCount < 15000 {
		t.Errorf("Expected at least 15,000 records, got %d", jsonCount)
	}

	// 7. Verification 2: Markdown-in-Markdown Integrity
	fullChunksDir := filepath.Join(chunksDir, "full", "2026-04-16")
	var totalCodeBlocks int
	codeBlockRegex := regexp.MustCompile("```[a-z]*\n")

	filepath.Walk(fullChunksDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			bytes, _ := os.ReadFile(path)
			totalCodeBlocks += len(codeBlockRegex.FindAllString(string(bytes), -1))
		}
		return nil
	})

	t.Logf("Found %d intact markdown code blocks in the full chunks.", totalCodeBlocks)
	if totalCodeBlocks < 500 {
		t.Errorf("Expected thousands of code blocks, got %d. Markdown parsing compromised.", totalCodeBlocks)
	}
}
