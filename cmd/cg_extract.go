package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Conversation struct {
	Title       string          `json:"title"`
	CreateTime  float64         `json:"create_time"`
	CurrentNode string          `json:"current_node"`
	Mapping     map[string]Node `json:"mapping"`
}

type Node struct {
	ID      string   `json:"id"`
	Message *Message `json:"message"`
	Parent  string   `json:"parent"`
}

type Message struct {
	Author struct {
		Role string `json:"role"`
	} `json:"author"`
	CreateTime float64 `json:"create_time"`
	Content    struct {
		Parts []interface{} `json:"parts"`
	} `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

func RunChatGPTExtract(sourceDir, exportDir string) {
	// Build Global Attachment Index
	var validAttachments []string
	absExportDir, _ := filepath.Abs(exportDir)
	entries, _ := os.ReadDir(exportDir)
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".html") {
			validAttachments = append(validAttachments, filepath.Join(absExportDir, entry.Name()))
		}
	}
	if len(validAttachments) > 0 {
		idxPath := filepath.Join(sourceDir, "0000_ChatGPT_Attachments_Index.md")
		os.MkdirAll(sourceDir, 0755)
		idxContent := "# ChatGPT Available Attachments from this Export\n\n"
		for _, att := range validAttachments {
			idxContent += fmt.Sprintf("- [Local Path: %s]\n", att)
		}
		os.WriteFile(idxPath, []byte(idxContent), 0644)
	}

	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "conversations") && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(exportDir, entry.Name()))
		}
	}

	totalInteractions := 0

	for _, jsonFile := range jsonFiles {
		file, err := os.Open(jsonFile)
		if err != nil { continue }
		bytes, _ := io.ReadAll(file)
		file.Close()

		var convos []Conversation
		json.Unmarshal(bytes, &convos)

		for _, conv := range convos {
			if conv.Title == "" || conv.CurrentNode == "" { continue }

			// Traverse DAG
			var linearTimeline []Node
			currID := conv.CurrentNode
			for currID != "" {
				node, exists := conv.Mapping[currID]
				if !exists { break }
				linearTimeline = append(linearTimeline, node)
				currID = node.Parent
			}

			// Reverse to chronological
			for i, j := 0, len(linearTimeline)-1; i < j; i, j = i+1, j-1 {
				linearTimeline[i], linearTimeline[j] = linearTimeline[j], linearTimeline[i]
			}

			// State Machine
			var currentPrompt string
			var currentResponseParts []string
			var promptTime time.Time
			var capturedAttachments []string
			var currentModel string = "unknown"

			writePair := func() {
				if currentPrompt == "" && len(currentResponseParts) == 0 { return }
				
				if promptTime.IsZero() { promptTime = time.Unix(int64(conv.CreateTime), 0) }

				year := fmt.Sprintf("%04d", promptTime.Year())
				month := fmt.Sprintf("%02d", promptTime.Month())
				timestamp := promptTime.Format("20060102T150405")

				safeSubject := makeSafeFilenameCG(conv.Title)
				if len(safeSubject) > 30 { safeSubject = safeSubject[:30] }
				if safeSubject == "" { safeSubject = "chat" }

				cleanTitle := strings.ReplaceAll(conv.Title, "\n", " ")

				filename := fmt.Sprintf("%s-%s.md", timestamp, safeSubject)
				targetPath := filepath.Join(sourceDir, year, month)
				os.MkdirAll(targetPath, 0755)

				mdContent := "## AI Interaction\n\n"
				mdContent += "### Date\n"
				mdContent += fmt.Sprintf("%s\n\n", promptTime.Format(time.RFC3339))
				mdContent += "### Source\n"
				mdContent += "ChatGPT\n\n"
				mdContent += "### Chat Name\n"
				mdContent += fmt.Sprintf("%s\n\n", cleanTitle)
				mdContent += "### Model\n"
				mdContent += fmt.Sprintf("%s\n\n", currentModel)

				mdContent += "### Prompt\n"
				if currentPrompt != "" {
					mdContent += fmt.Sprintf("%s\n\n", currentPrompt)
				} else {
					mdContent += "*(No text prompt)*\n\n"
				}

				mdContent += "### Response\n"
				if len(currentResponseParts) > 0 {
					mdContent += fmt.Sprintf("%s\n\n", strings.Join(currentResponseParts, "\n\n---\n\n"))
				} else {
					mdContent += "*(No response text recorded)*\n\n"
				}

				if len(capturedAttachments) > 0 {
					mdContent += "### Attachments Used\n"
					for _, att := range capturedAttachments {
						mdContent += fmt.Sprintf("- *(Attached file: %s)*\n", att)
					}
					mdContent += "\n"
				}

				os.WriteFile(filepath.Join(targetPath, filename), []byte(mdContent), 0644)
				totalInteractions++

				// Reset state
				currentPrompt = ""
				currentResponseParts = nil
				capturedAttachments = nil
				currentModel = "unknown"
			}

			for _, node := range linearTimeline {
				if node.Message == nil || node.Message.Author.Role == "system" { continue }

				role := node.Message.Author.Role
				var textParts []string
				for _, part := range node.Message.Content.Parts {
					if strPart, ok := part.(string); ok { textParts = append(textParts, strPart) }
				}
				text := strings.Join(textParts, "\n")

				if node.Message.Metadata != nil {
					if m, ok := node.Message.Metadata["model_slug"].(string); ok { currentModel = m }
					
					if files, ok := node.Message.Metadata["files"].([]interface{}); ok {
						for _, f := range files {
							if fMap, ok := f.(map[string]interface{}); ok {
								if name, ok := fMap["name"].(string); ok { capturedAttachments = append(capturedAttachments, name) }
							}
						}
					}
					if atts, ok := node.Message.Metadata["attachments"].([]interface{}); ok {
						for _, a := range atts {
							if aMap, ok := a.(map[string]interface{}); ok {
								if name, ok := aMap["name"].(string); ok { capturedAttachments = append(capturedAttachments, name) }
							}
						}
					}
				}

				if role == "user" {
					writePair() // Flush previous turn
					currentPrompt = text
					if node.Message.CreateTime > 0 { promptTime = time.Unix(int64(node.Message.CreateTime), 0) }
				} else if role == "assistant" || role == "tool" {
					if text != "" { currentResponseParts = append(currentResponseParts, text) }
				}
			}
			writePair() // Flush final turn
		}
	}

	fmt.Printf("[✓] Extracted %d ChatGPT interactions directly to %s/\n", totalInteractions, sourceDir)
}

func makeSafeFilenameCG(str string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	return strings.Trim(reg.ReplaceAllString(str, "_"), "_")
}
