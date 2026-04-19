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

	"github.com/peterjbraam/ggo/pb/v1"
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
	var validAttachments []string
	var jsonPath string

	filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := info.Name()
		if name == "MyActivity.json" {
			jsonPath = path
		} else if !strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".html") {
			absPath, _ := filepath.Abs(path)
			validAttachments = append(validAttachments, absPath)
		}
		return nil
	})

	if jsonPath == "" {
		fmt.Printf("[!] Could not find MyActivity.json in %s, skipping...\n", exportDir)
		return
	}

	file, _ := os.Open(jsonPath)
	defer file.Close()
	bytes, _ := io.ReadAll(file)

	var records []GeminiRecord
	json.Unmarshal(bytes, &records)

	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	count := 0

	for _, rec := range records {
		if !strings.HasPrefix(rec.Title, "Prompted ") {
			continue
		}

		prompt := strings.TrimPrefix(rec.Title, "Prompted ")
		promptTime, err := time.Parse(time.RFC3339, rec.Time)
		if err != nil {
			continue
		}

		promptTime = promptTime.UTC()

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
		timestamp := promptTime.Format("20060102T150405Z")

		safeSubject := MakeSafeSlug(prompt)
		filename := fmt.Sprintf("%s-%s.json", timestamp, safeSubject)
		targetPath := filepath.Join(sourceDir, year, month)

		record := &pb.AiRecord{
			Metadata: &pb.Metadata{
				TimestampUtc: promptTime.Format(time.RFC3339),
				ChatId:       "gemini-takeout",
				ModelName:    "gemini-unknown",
			},
			Input: &pb.UserInput{
				Narrative: prompt,
			},
			Output: &pb.AIResponse{
				Narrative: responseText,
			},
		}

		// Process legacy attachments if found in the Takeout bundle
		for _, att := range validAttachments {
			record.Input.Attachments = append(record.Input.Attachments, &pb.Attachment{
				FileName:       filepath.Base(att),
				LocalVaultPath: att,
			})
		}

		WriteRecord(targetPath, filename, record)
		count++
	}

	fmt.Printf("[✓] Extracted %d Gemini interactions to strictly typed JSON in %s/\n", count, sourceDir)
}
