package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atotto/clipboard"
	"github.com/spf13/viper"
)

func TestAutoCopyToClipboardUnit(t *testing.T) {
	// 1. Polite Environment Handling
	originalClip, err := clipboard.ReadAll()
	if err != nil {
		t.Skipf("Skipping unit clipboard test, OS clipboard not accessible: %v", err)
	}
	defer clipboard.WriteAll(originalClip)

	// Use hex codes for backticks (\x60) so markdown parsers don't break the code block visually
	ticks := "\x60\x60\x60"

	// 2. Prepare mock LLM response
	// We use single backslashes here so Go injects actual byte 27 (ESC) and byte 0 (NULL).
	mockResponse := "Here is the first step you need to run:\n" +
		ticks + "bash\ncd /var/log\nls -la\n" + ticks + "\n\n" +
		"Then apply this Go patch (contains a hidden null byte and ANSI color):\n" +
		ticks + "go\nfmt.Println(\"\x1b[31mError\x00\")\n" + ticks + "\n\n" +
		"Update your configuration:\n" +
		ticks + "yaml\nserver:\n  port: 8080\n" + ticks + "\n\n" +
		"Finally, restart the service:\n" +
		ticks + "\nsudo systemctl restart ggo\n" + ticks + "\n\n" +
		"Let me know if you need more help!\n"

	// 3. Define the exact expected string in the clipboard
	// Note: We expect the markdown ticks and conversational text to vanish.
	// The actual \x00 and \x1b should be stripped out (leaving the naive "[31m" behind).
	expectedClipboard := "cd /var/log\nls -la\n\nfmt.Println(\"[31mError\")\n\nserver:\n  port: 8080\n\nsudo systemctl restart ggo"

	// 4. Action
	autoCopyToClipboard(mockResponse)

	// 5. Verification
	actualClip, err := clipboard.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read clipboard after action: %v", err)
	}

	if actualClip != expectedClipboard {
		t.Errorf("Clipboard content did not match expected.\n--- EXPECTED ---\n%s\n--- ACTUAL ---\n%s\n", expectedClipboard, actualClip)
	}
}

func TestLiveAutoCopyToClipboard(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("Skipping live clipboard API test: GEMINI_API_KEY environment variable is not set.")
	}

	originalClip, err := clipboard.ReadAll()
	if err != nil {
		t.Skipf("Skipping live clipboard test, OS clipboard not accessible: %v", err)
	}
	defer clipboard.WriteAll(originalClip)

	// 1. Sandbox the environment
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	ProjectViper = viper.New()
	GlobalViper = viper.New()
	Project.ProjectName = "live_clipboard_test"
	ActiveChatName = "clipboard_chat"

	// Create required directories so UpdateChatModel can write GEMINI.md
	projDir := filepath.Join(tempHome, ".ggo", "ggoai-live_clipboard_test")
	os.MkdirAll(filepath.Join(projDir, "chats"), 0755)

	// 2. Initialize real Gemini API
	ctx := context.Background()
	err = InitGemini(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize Gemini: %v", err)
	}

	// 3. Action: Send a prompt enforcing strict formatting behavior
	prompt := "I need to test your snippet formatting. \n" +
		"Please provide exactly two commands: \n" +
		"1. A bash command that prints the word 'hello'.\n" +
		"2. A bash command that prints the word 'world'.\n" +
		"Do not combine them into one block. Give each its own markdown code block."

	// Send it. This explicitly hits the Gemini API and triggers autoCopyToClipboard
	sendAndRender(ctx, prompt)

	// 4. Verification
	actualClip, err := clipboard.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read clipboard after API call: %v", err)
	}

	// Because LLMs can be slightly non-deterministic, we check for structural invariants.
	if !strings.Contains(actualClip, "hello") {
		t.Errorf("Clipboard is missing the first requested command payload. Got:\n%s", actualClip)
	}
	if !strings.Contains(actualClip, "world") {
		t.Errorf("Clipboard is missing the second requested command payload. Got:\n%s", actualClip)
	}

	// Use hex string here so the web UI doesn't panic and truncate
	if strings.Contains(actualClip, "\x60\x60\x60") {
		t.Errorf("Clipboard incorrectly contains markdown block markers. They were not stripped.\nGot: %s", actualClip)
	}
	if strings.Contains(strings.ToLower(actualClip), "here is") || strings.Contains(strings.ToLower(actualClip), "sure") {
		t.Errorf("Clipboard seems to contain conversational fluff, meaning the regex extraction failed or the LLM broke the invariant.\nGot: %s", actualClip)
	}
}
