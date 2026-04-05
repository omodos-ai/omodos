package cmd

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GeminiRecord struct {
	Title        string `json:"title"`
	Time         string `json:"time"`
	Content      string `json:"content"`
	SafeHtmlItem []struct {
		Html string `json:"html"`
	} `json:"safeHtmlItem"`
}

func RunGeminiExtract(sourceDir, exportDir string) {
	geminiAppsDir := filepath.Join(exportDir, "My Activity", "Gemini Apps")
	jsonPath := filepath.Join(geminiAppsDir, "MyActivity.json")

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		fmt.Printf("[!] Could not find %s, skipping...\n", jsonPath)
		return
	}

	// Build Global Attachment Index (ONCE)
	var validAttachments []string
	absGeminiDir, _ := filepath.Abs(geminiAppsDir)
	entries, _ := os.ReadDir(geminiAppsDir)
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".html") {
			validAttachments = append(validAttachments, filepath.Join(absGeminiDir, entry.Name()))
		}
	}
	if len(validAttachments) > 0 {
		idxPath := filepath.Join(sourceDir, "0000_Gemini_Attachments_Index.md")
		os.MkdirAll(sourceDir, 0755)
		idxContent := "# Gemini Available Attachments from this Export\n\n"
		for _, att := range validAttachments {
			idxContent += fmt.Sprintf("- [Local Path: %s]\n", att)
		}
		os.WriteFile(idxPath, []byte(idxContent), 0644)
	}

	file, _ := os.Open(jsonPath)
	defer file.Close()
	bytes, _ := io.ReadAll(file)

	var records []GeminiRecord
	json.Unmarshal(bytes, &records)

	// Reverse records to be strictly chronological (oldest to newest)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	count := 0

	// Traverse timeline
	for _, rec := range records {
		if !strings.HasPrefix(rec.Title, "Prompted ") {
			continue // Skip non-conversational events
		}

		prompt := strings.TrimPrefix(rec.Title, "Prompted ")
		promptTime, err := time.Parse(time.RFC3339, rec.Time)
		if err != nil {
			continue
		}

		var responseText string
		if len(rec.SafeHtmlItem) > 0 {
			rawHtml := rec.SafeHtmlItem[0].Html
			decoded := html.UnescapeString(rawHtml)
			decoded = strings.ReplaceAll(decoded, "<h3>", "### ")
			decoded = strings.ReplaceAll(decoded, "</h3>", "\n")
			decoded = strings.ReplaceAll(decoded, "<h2>", "## ")
			decoded = strings.ReplaceAll(decoded, "</h2>", "\n")
			decoded = strings.ReplaceAll(decoded, "<p>", "")
			decoded = strings.ReplaceAll(decoded, "</p>", "\n\n")
			decoded = strings.ReplaceAll(decoded, "<li>", "* ")
			decoded = strings.ReplaceAll(decoded, "</li>", "\n")
			decoded = strings.ReplaceAll(decoded, "<strong>", "**")
			decoded = strings.ReplaceAll(decoded, "</strong>", "**")
			decoded = strings.ReplaceAll(decoded, "<em>", "*")
			decoded = strings.ReplaceAll(decoded, "</em>", "*")
			decoded = strings.ReplaceAll(decoded, "<br>", "\n")

			re := regexp.MustCompile(`<[^>]*>`)
			responseText = re.ReplaceAllString(decoded, "")
			responseText = strings.TrimSpace(responseText)

		} else if rec.Content != "" {
			responseText = rec.Content
		} else {
			responseText = "*(No response text recorded)*"
		}

		year := fmt.Sprintf("%04d", promptTime.Year())
		month := fmt.Sprintf("%02d", promptTime.Month())
		timestamp := promptTime.Format("20060102T150405")

		safeSubject := makeSafeFilenameGemini(prompt)
		if len(safeSubject) > 30 {
			safeSubject = safeSubject[:30]
		}
		if safeSubject == "" {
			safeSubject = "chat"
		}

		cleanTitle := strings.ReplaceAll(prompt, "\n", " ")
		if len(cleanTitle) > 50 {
			cleanTitle = cleanTitle[:50] + "..."
		}

		filename := fmt.Sprintf("%s-%s.md", timestamp, safeSubject)
		targetPath := filepath.Join(sourceDir, year, month)
		os.MkdirAll(targetPath, 0755)

		mdContent := "## AI Interaction\n\n"
		mdContent += "### Date\n"
		mdContent += fmt.Sprintf("%s\n\n", promptTime.Format(time.RFC3339))
		mdContent += "### Source\n"
		mdContent += "Gemini\n\n"
		mdContent += "### Subject\n"
		mdContent += fmt.Sprintf("%s\n\n", cleanTitle)
		mdContent += "### Prompt\n"
		mdContent += fmt.Sprintf("%s\n\n", prompt)
		mdContent += "### Response\n"
		mdContent += fmt.Sprintf("%s\n\n", responseText)

		os.WriteFile(filepath.Join(targetPath, filename), []byte(mdContent), 0644)
		count++
	}

	fmt.Printf("[✓] Extracted %d Gemini interactions directly to %s/\n", count, sourceDir)
}

func makeSafeFilenameGemini(str string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	return strings.Trim(reg.ReplaceAllString(str, "_"), "_")
}
