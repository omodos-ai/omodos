package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/pb/v1"
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
	var jsonFiles []string

	filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), "conversations") && strings.HasSuffix(info.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})

	totalInteractions := 0

	for _, jsonFile := range jsonFiles {
		file, err := os.Open(jsonFile)
		if err != nil {
			continue
		}
		bytes, _ := io.ReadAll(file)
		file.Close()

		var convos []Conversation
		json.Unmarshal(bytes, &convos)

		for _, conv := range convos {
			if conv.Title == "" || conv.CurrentNode == "" {
				continue
			}

			var linearTimeline []Node
			currID := conv.CurrentNode
			for currID != "" {
				node, exists := conv.Mapping[currID]
				if !exists {
					break
				}
				linearTimeline = append(linearTimeline, node)
				currID = node.Parent
			}

			for i, j := 0, len(linearTimeline)-1; i < j; i, j = i+1, j-1 {
				linearTimeline[i], linearTimeline[j] = linearTimeline[j], linearTimeline[i]
			}

			var currentPrompt string
			var currentResponseParts []string
			var promptTime time.Time
			var capturedAttachments []string
			var currentModel string = "unknown"

			writePair := func() {
				if currentPrompt == "" && len(currentResponseParts) == 0 {
					return
				}

				if promptTime.IsZero() {
					promptTime = time.Unix(int64(conv.CreateTime), 0).UTC()
				}

				year := fmt.Sprintf("%04d", promptTime.Year())
				month := fmt.Sprintf("%02d", promptTime.Month())
				timestamp := promptTime.Format("20060102T150405Z")

				safeSubject := MakeSafeSlug(conv.Title)
				filename := fmt.Sprintf("%s-%s.json", timestamp, safeSubject)
				targetPath := filepath.Join(sourceDir, year, month)

				record := &pb.AiRecord{
					Metadata: &pb.Metadata{
						TimestampUtc: promptTime.Format(time.RFC3339),
						ChatId:       conv.Title,
						ModelName:    currentModel,
					},
					Input: &pb.UserInput{
						Narrative: currentPrompt,
					},
					Output: &pb.AIResponse{
						Narrative: strings.Join(currentResponseParts, "\n\n"),
					},
				}

				for _, att := range capturedAttachments {
					record.Input.Attachments = append(record.Input.Attachments, &pb.Attachment{FileName: att})
				}

				WriteRecord(targetPath, filename, record)
				totalInteractions++

				currentPrompt = ""
				currentResponseParts = nil
				capturedAttachments = nil
				currentModel = "unknown"
			}

			for _, node := range linearTimeline {
				if node.Message == nil || node.Message.Author.Role == "system" {
					continue
				}

				role := node.Message.Author.Role
				var textParts []string
				for _, part := range node.Message.Content.Parts {
					if strPart, ok := part.(string); ok {
						textParts = append(textParts, strPart)
					}
				}
				text := strings.Join(textParts, "\n")

				if node.Message.Metadata != nil {
					if m, ok := node.Message.Metadata["model_slug"].(string); ok {
						currentModel = m
					}

					extractAttachments := func(key string) {
						if files, ok := node.Message.Metadata[key].([]interface{}); ok {
							for _, f := range files {
								if fMap, ok := f.(map[string]interface{}); ok {
									if name, ok := fMap["name"].(string); ok {
										capturedAttachments = append(capturedAttachments, name)
									}
								}
							}
						}
					}
					extractAttachments("files")
					extractAttachments("attachments")
				}

				if role == "user" {
					writePair()
					currentPrompt = text
					if node.Message.CreateTime > 0 {
						promptTime = time.Unix(int64(node.Message.CreateTime), 0).UTC()
					}
				} else if role == "assistant" || role == "tool" {
					if text != "" {
						currentResponseParts = append(currentResponseParts, text)
					}
				}
			}
			writePair()
		}
	}

	fmt.Printf("[✓] Extracted %d ChatGPT interactions to strictly typed JSON in %s/\n", totalInteractions, sourceDir)
}
