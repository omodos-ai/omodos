package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
)

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

// cleanForClipboard extracts purely the code from markdown blocks if present,
// otherwise it strips basic markdown to return raw text.
func cleanForClipboard(text string) string {
	// 1. If there are code blocks, extract ONLY the code.
	re := regexp.MustCompile(`(?s)` + "```" + `[a-zA-Z0-9]*\n(.*?)\n` + "```")
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		var sb strings.Builder
		for _, m := range matches {
			sb.WriteString(m[1] + "\n\n")
		}
		return strings.TrimSpace(sb.String())
	}
	
	// 2. If no code blocks, strip standard markdown formatting roughly
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "### ", "")
	text = strings.ReplaceAll(text, "## ", "")
	text = strings.ReplaceAll(text, "# ", "")
	return strings.TrimSpace(text)
}
