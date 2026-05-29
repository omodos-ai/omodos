package ggo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/peterjbraam/ggo/internal/core"
)

// ApplyMacroEdits scans the provided text for the strict XML <file> schema,
// extracts the payloads, and writes them to disk inside a Time Machine transaction.
func ApplyMacroEdits(responseText string) (int, error) {
	// Match <file path="x" [optional attributes]>content</file> across multiple lines
	re := regexp.MustCompile(`(?s)<file\s+path="([^"]+)"[^>]*>(.*?)</file>`)
	matches := re.FindAllStringSubmatch(responseText, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("no <file> blocks found in the response")
	}

	// Wrap the entire overwrite batch in a single Time Machine transaction
	err := core.ExecuteTransaction("write_files", func(anchor string, now time.Time) error {
		for _, match := range matches {
			path := match[1]
			content := match[2]

			// The LLM almost always adds a leading/trailing newline inside the XML tags.
			// We strip exactly one of each (if present) so we don't bloat the files.
			content = strings.TrimPrefix(content, "\n")
			content = strings.TrimSuffix(content, "\n")

			// Ensure the target directory structure exists
			err := os.MkdirAll(filepath.Dir(path), 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory for %s: %v", path, err)
			}

			err = os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return fmt.Errorf("failed to write %s: %v", path, err)
			}

			fmt.Printf("\033[32m[✓] Overwrote:\033[0m %s\n", path)
		}
		return nil
	})

	return len(matches), err
}

func handleWF() {
	if lastAPIResponse == "" {
		fmt.Println("[!] No recent response available to extract files from.")
		return
	}

	fmt.Println("Scanning last response for macro-edits...")
	count, err := ApplyMacroEdits(lastAPIResponse)
	if err != nil {
		fmt.Printf("\033[31m[Error]\033[0m %v\n", err)
	} else {
		fmt.Printf("[Info] Successfully applied %d file modification(s).\n", count)
	}
}

func handleCR() {
	if lastAPIResponse == "" {
		fmt.Println("[!] No recent response to copy.")
		return
	}

	textToCopy := cleanForClipboard(lastAPIResponse)
	err := clipboard.WriteAll(textToCopy)
	if err != nil {
		fmt.Printf("[Error] Cannot write to OS clipboard: %v\n", err)
	} else {
		fmt.Println("[✓] Response copied to clipboard (ready to paste).")
	}
}

func handleCW(parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: cw <path>")
		return
	}
	if lastAPIResponse == "" {
		fmt.Println("[!] No recent response to write.")
		return
	}

	path := parts[1]
	textToWrite := cleanForClipboard(lastAPIResponse)

	err := os.WriteFile(path, []byte(textToWrite), 0644)
	if err != nil {
		fmt.Printf("[Error] Failed to write file: %v\n", err)
	} else {
		fmt.Printf("[✓] Response safely written to %s\n", path)
	}
}

// / cleanForClipboard extracts purely the code from markdown blocks if present,
// otherwise it strips basic markdown to return raw text, applying strict sanitization.
func cleanForClipboard(text string) string {
	// 1. If there are code blocks, extract ONLY the code.
	ticks := "\x60\x60\x60"
	re := regexp.MustCompile(`(?s)` + ticks + `[a-zA-Z0-9]*\n(.*?)\n` + ticks)
	matches := re.FindAllStringSubmatch(text, -1)

	if len(matches) > 0 {
		var sb strings.Builder
		for _, m := range matches {
			snippet := m[1]
			// Strip dangerous control characters from the snippet
			snippet = strings.ReplaceAll(snippet, "\x00", "")
			snippet = strings.ReplaceAll(snippet, "\x1b", "")
			sb.WriteString(snippet + "\n\n")
		}
		return strings.TrimSpace(sb.String())
	}

	// 2. If no code blocks, strip standard markdown formatting roughly
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "### ", "")
	text = strings.ReplaceAll(text, "## ", "")
	text = strings.ReplaceAll(text, "# ", "")

	// Strip dangerous control characters from the fallback text
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.ReplaceAll(text, "\x1b", "")

	return strings.TrimSpace(text)
}
