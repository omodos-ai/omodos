package raggo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
)

type ChatGPTExport []ChatGPTConversation

type ChatGPTConversation struct {
	Title       string                 `json:"title"`
	CreateTime  float64                `json:"create_time"`
	CurrentNode string                 `json:"current_node"`
	Mapping     map[string]ChatGPTNode `json:"mapping"`
}

type ChatGPTNode struct {
	ID      string          `json:"id"`
	Message *ChatGPTMessage `json:"message"`
	Parent  string          `json:"parent"`
}

type ChatGPTMessage struct {
	Author     ChatGPTAuthor  `json:"author"`
	CreateTime float64        `json:"create_time"`
	Content    ChatGPTContent `json:"content"`
}

type ChatGPTAuthor struct {
	Role string `json:"role"`
}

type ChatGPTContent struct {
	Parts []interface{} `json:"parts"`
}

func RunChatGPTExtract(recordsDir, exportDir string, overwrite bool) {
	jsonPaths := findChatGPTShards(exportDir)
	if len(jsonPaths) == 0 {
		fmt.Printf("[!] Could not find any conversations.json shards in %s, skipping...\n", exportDir)
		return
	}

	totalCount := 0
	totalSkipped := 0
	for _, jsonPath := range jsonPaths {
		written, skipped := processChatGPTShard(jsonPath, recordsDir, overwrite)
		totalCount += written
		totalSkipped += skipped
	}

	fmt.Printf("[✓] Extracted %d ChatGPT interactions from %d shard(s) to JSON ledger (Skipped: %d).\n", totalCount, len(jsonPaths), totalSkipped)
}

func findChatGPTShards(exportDir string) []string {
	var paths []string
	filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, "conversations") && strings.HasSuffix(name, ".json") {
				paths = append(paths, path)
			}
		}
		return nil
	})
	return paths
}

func processChatGPTShard(jsonPath, recordsDir string, overwrite bool) (int, int) {
	bytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return 0, 0
	}

	var conversations ChatGPTExport
	if err := json.Unmarshal(bytes, &conversations); err != nil {
		return 0, 0
	}

	writtenCount := 0
	skippedCount := 0
	for _, conv := range conversations {
		w, s := processChatGPTConversation(conv, recordsDir, overwrite)
		writtenCount += w
		skippedCount += s
	}
	return writtenCount, skippedCount
}

func processChatGPTConversation(conv ChatGPTConversation, recordsDir string, overwrite bool) (int, int) {
	chatTitle := conv.Title
	if chatTitle == "" {
		chatTitle = "Untitled Chat"
	}

	activeTimeline := buildConversationTimeline(conv)
	written := 0
	skipped := 0
	var currentPrompt string
	var promptTime time.Time

	for _, node := range activeTimeline {
		if node.Message == nil {
			continue
		}

		text := extractMessageText(node.Message)
		if text == "" {
			continue
		}

		role := node.Message.Author.Role
		if role == "user" {
			currentPrompt = text
			promptTime = time.Unix(int64(node.Message.CreateTime), 0).UTC()
			continue
		}

		if role == "assistant" && currentPrompt != "" {
			if writeChatGPTRecord(recordsDir, chatTitle, currentPrompt, text, promptTime, overwrite) {
				written++
			} else {
				skipped++
			}
			currentPrompt = ""
		}
	}
	return written, skipped
}

func buildConversationTimeline(conv ChatGPTConversation) []*ChatGPTNode {
	var timeline []*ChatGPTNode
	curr := conv.CurrentNode

	for curr != "" {
		node, exists := conv.Mapping[curr]
		if !exists {
			break
		}
		timeline = append(timeline, &node)
		curr = node.Parent
	}

	for i, j := 0, len(timeline)-1; i < j; i, j = i+1, j-1 {
		timeline[i], timeline[j] = timeline[j], timeline[i]
	}
	return timeline
}

func extractMessageText(msg *ChatGPTMessage) string {
	var textParts []string
	for _, part := range msg.Content.Parts {
		if str, ok := part.(string); ok {
			textParts = append(textParts, str)
		}
	}
	return strings.TrimSpace(strings.Join(textParts, "\n"))
}

func writeChatGPTRecord(recordsDir, chatTitle, prompt, response string, promptTime time.Time, overwrite bool) bool {
	year := fmt.Sprintf("%04d", promptTime.Year())
	month := fmt.Sprintf("%02d", promptTime.Month())
	timestamp := promptTime.Format("20060102T150405Z")
	safeSubject := core.MakeSafeSlug(prompt)
	filename := fmt.Sprintf("%s-cg-%s.json", timestamp, safeSubject)

	targetDir := filepath.Join(recordsDir, year, month)
	fullPath := filepath.Join(targetDir, filename)

	if !overwrite {
		if _, err := os.Stat(fullPath); err == nil {
			return false
		}
	}

	record := &pb.AiRecord{
		Metadata: &pb.Metadata{
			TimestampUtc: promptTime.Format(time.RFC3339),
			ChatId:       chatTitle,
			ModelName:    "chatgpt-legacy",
		},
		Input: &pb.UserInput{
			Narrative: prompt,
		},
		Output: &pb.AIResponse{
			Narrative: response,
		},
	}

	core.WriteRecord(targetDir, filename, record)
	return true
}
