package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/cobra"
)

var cgPrRespCmd = &cobra.Command{
	Use:   "cg-pr-resp",
	Short: "Parses ChatGPT JSON exports into the markdown ledger",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if Global.SourceDir == "" || Global.IngestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir and ingest_dir are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "chatgpt")
		fmt.Printf("Starting ChatGPT Parse Pipeline...\n")
		fmt.Printf("Source Dir: %s\n", Global.SourceDir)
		fmt.Printf("Ingest Dir: %s\n", exportDir)

		RunChatGPTExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(cgPrRespCmd)
}

// Structs kept exactly as they were in your original parser
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

func RunChatGPTExtract(sourceDir, exportDir string) {
	jsonPaths := findChatGPTShards(exportDir)
	if len(jsonPaths) == 0 {
		fmt.Printf("[!] Could not find any conversations.json shards in %s, skipping...\n", exportDir)
		return
	}

	totalCount := 0
	for _, jsonPath := range jsonPaths {
		totalCount += processChatGPTShard(jsonPath, sourceDir)
	}

	fmt.Printf("[✓] Safely extracted %d ChatGPT interactions from %d shard(s) to full and cognitive ledgers.\n", totalCount, len(jsonPaths))
}

// -----------------------------------------------------------------------------
// Extraction Helpers
// -----------------------------------------------------------------------------

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

func processChatGPTShard(jsonPath, sourceDir string) int {
	bytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return 0
	}

	var conversations ChatGPTExport
	if err := json.Unmarshal(bytes, &conversations); err != nil {
		return 0
	}

	shardCount := 0
	for _, conv := range conversations {
		shardCount += processChatGPTConversation(conv, sourceDir)
	}
	return shardCount
}

func processChatGPTConversation(conv ChatGPTConversation, sourceDir string) int {
	chatTitle := conv.Title
	if chatTitle == "" {
		chatTitle = "Untitled Chat"
	}

	activeTimeline := buildConversationTimeline(conv)
	count := 0
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
			writeChatGPTRecord(sourceDir, chatTitle, currentPrompt, text, promptTime)
			count++
			currentPrompt = "" // Reset for the next conversational turn
		}
	}
	return count
}

func buildConversationTimeline(conv ChatGPTConversation) []*ChatGPTNode {
	var timeline []*ChatGPTNode
	curr := conv.CurrentNode

	// Walk backwards up the graph
	for curr != "" {
		node, exists := conv.Mapping[curr]
		if !exists {
			break
		}
		timeline = append(timeline, &node)
		curr = node.Parent
	}

	// Reverse to chronological order
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

func writeChatGPTRecord(sourceDir, chatTitle, prompt, response string, promptTime time.Time) {
	year := fmt.Sprintf("%04d", promptTime.Year())
	month := fmt.Sprintf("%02d", promptTime.Month())
	timestamp := promptTime.Format("20060102T150405Z")
	safeSubject := MakeSafeSlug(prompt)
	filename := fmt.Sprintf("%s-cg-%s.json", timestamp, safeSubject)

	// 1. Build the Full Record
	fullRecord := &pb.AiRecord{
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

	// 2. Clone and strip for the Cognitive Record (Identical to Gemini behavior)
	cogRecord := proto.Clone(fullRecord).(*pb.AiRecord)
	cogRecord.Input.Narrative = stripLargeCodeBlocks(cogRecord.Input.Narrative)
	cogRecord.Output.Narrative = stripLargeCodeBlocks(cogRecord.Output.Narrative)

	fullPath := filepath.Join(sourceDir, "ai-records", "full", year, month)
	cogPath := filepath.Join(sourceDir, "ai-records", "cognitive", year, month)

	// 3. Write to both ledgers
	WriteRecord(fullPath, filename, fullRecord)
	WriteRecord(cogPath, filename, cogRecord)
}

