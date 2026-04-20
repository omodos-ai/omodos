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
	var jsonPath string

	filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "conversations.json" {
			jsonPath = path
		}
		return nil
	})

	if jsonPath == "" {
		fmt.Printf("[!] Could not find conversations.json in %s, skipping...\n", exportDir)
		return
	}

	file, _ := os.Open(jsonPath)
	defer file.Close()
	bytes, _ := io.ReadAll(file)

	var conversations ChatGPTExport
	json.Unmarshal(bytes, &conversations)

	count := 0

	// YOUR EXACT GRAPH WALKING LOGIC
	for _, conv := range conversations {
		chatTitle := conv.Title
		if chatTitle == "" {
			chatTitle = "Untitled Chat"
		}

		var activeTimeline []*ChatGPTNode
		curr := conv.CurrentNode

		for curr != "" {
			node, exists := conv.Mapping[curr]
			if !exists {
				break
			}
			activeTimeline = append(activeTimeline, &node)
			curr = node.Parent
		}

		for i, j := 0, len(activeTimeline)-1; i < j; i, j = i+1, j-1 {
			activeTimeline[i], activeTimeline[j] = activeTimeline[j], activeTimeline[i]
		}

		var currentPrompt string
		var promptTime time.Time

		for _, node := range activeTimeline {
			if node.Message == nil {
				continue
			}

			role := node.Message.Author.Role
			var textParts []string
			for _, part := range node.Message.Content.Parts {
				if str, ok := part.(string); ok {
					textParts = append(textParts, str)
				}
			}
			text := strings.Join(textParts, "\n")
			text = strings.TrimSpace(text)

			if text == "" {
				continue
			}

			if role == "user" {
				currentPrompt = text
				promptTime = time.Unix(int64(node.Message.CreateTime), 0).UTC()
			} else if role == "assistant" && currentPrompt != "" {

				// =====================================================================
				// STRICT MODIFICATION BOUNDARY: ONLY EMITTING JSON RECORDS BEYOND HERE
				// =====================================================================

				year := fmt.Sprintf("%04d", promptTime.Year())
				month := fmt.Sprintf("%02d", promptTime.Month())
				timestamp := promptTime.Format("20060102T150405Z")

				safeSubject := MakeSafeSlug(currentPrompt)
				filename := fmt.Sprintf("%s-cg-%s.json", timestamp, safeSubject)

				// Construct the identical AiRecord for both full and cognitive
				record := &pb.AiRecord{
					Metadata: &pb.Metadata{
						TimestampUtc: promptTime.Format(time.RFC3339),
						ChatId:       chatTitle, // Injected ChatGPT Title
						ModelName:    "chatgpt-legacy",
					},
					Input: &pb.UserInput{
						Narrative: currentPrompt,
					},
					Output: &pb.AIResponse{
						Narrative: text,
					},
				}

				// Write to BOTH ledgers with identically structured, unstripped records
				fullPath := filepath.Join(sourceDir, "ai-records", "full", year, month)
				cogPath := filepath.Join(sourceDir, "ai-records", "cognitive", year, month)

				WriteRecord(fullPath, filename, record)
				WriteRecord(cogPath, filename, record)

				count++
				currentPrompt = "" // Reset for the next turn in the graph
			}
		}
	}

	fmt.Printf("[✓] Safely extracted %d ChatGPT interactions to identical full and cognitive JSON ledgers.\n", count)
}
