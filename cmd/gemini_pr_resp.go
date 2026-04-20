package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/peterjbraam/ggo/pb/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var geminiPrRespCmd = &cobra.Command{
	Use:   "gemini-pr-resp",
	Short: "Parses Gemini JSON exports into full and cognitive ai-records ledgers",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if Global.SourceDir == "" || Global.IngestDir == "" {
			fmt.Println("[Error] Missing global config. Ensure source_dir and ingest_dir are set.")
			os.Exit(1)
		}

		exportDir := filepath.Join(Global.IngestDir, "gemini")
		fmt.Printf("Starting Strict Gemini Parse Pipeline...\n")

		RunGeminiExtract(Global.SourceDir, exportDir)
	},
}

func init() {
	rootCmd.AddCommand(geminiPrRespCmd)
}

func RunGeminiExtract(sourceVault, exportDir string) {
	validAttachments := make(map[string]string)
	var jsonPath string

	// 1. Catalog physical files on disk
	filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == "MyActivity.json" {
			jsonPath = path
		} else if !strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".html") {
			absPath, _ := filepath.Abs(path)
			validAttachments[name] = absPath
		}
		return nil
	})

	if jsonPath == "" {
		fmt.Printf("[!] Could not find MyActivity.json in %s, skipping...\n", exportDir)
		return
	}

	rawBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Printf("[Error] Failed to read %s: %v\n", jsonPath, err)
		return
	}

	// Wrap the array to satisfy protojson
	wrappedJSON := append([]byte(`{"activities": `), rawBytes...)
	wrappedJSON = append(wrappedJSON, []byte(`}`)...)

	var export pb.GeminiTakeoutExport

	// THE TRIPWIRE: DiscardUnknown is FALSE.
	// If Google alters the schema, it instantly fails.
	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: false}
	if err := unmarshaler.Unmarshal(wrappedJSON, &export); err != nil {
		fmt.Printf("\n[FATAL STRICT PARSE ERROR]\n")
		fmt.Printf("Google Takeout schema has changed. An undocumented JSON field was detected.\n")
		fmt.Printf("System halted to prevent data corruption.\n")
		fmt.Printf("-> ACTION REQUIRED: Call omodos.ai to get a fix.\n\n")
		fmt.Printf("Raw Error: %v\n", err)
		os.Exit(1)
	}

	// Process chronologically
	activities := export.Activities
	for i, j := 0, len(activities)-1; i < j; i, j = i+1, j-1 {
		activities[i], activities[j] = activities[j], activities[i]
	}

	converter := md.NewConverter("", true, nil)
	count := 0

	for _, rec := range activities {
		if !strings.HasPrefix(rec.Title, "Prompted ") {
			continue
		}

		prompt := strings.TrimPrefix(rec.Title, "Prompted ")
		promptTime, err := time.Parse(time.RFC3339, rec.Time)
		if err != nil {
			continue
		}

		promptTime = promptTime.UTC()

		// 2. Safe AST HTML -> Markdown Conversion
		var responseText string
		if len(rec.SafeHtmlItem) > 0 {
			markdownText, err := converter.ConvertString(rec.SafeHtmlItem[0].Html)
			if err != nil {
				responseText = "*(Error converting HTML to Markdown)*"
			} else {
				responseText = strings.TrimSpace(markdownText)
			}
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

		// 3. Construct the Central Full AiRecord
		fullRecord := &pb.AiRecord{
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

		// Intelligent Attachment Deduplication
		attSet := make(map[string]bool)
		for _, f := range rec.AttachedFiles {
			attSet[f] = true
		}
		if rec.ImageFile != "" {
			attSet[rec.ImageFile] = true
		}
		for _, sub := range rec.Subtitles {
			if sub.Url != "" {
				attSet[sub.Url] = true
			}
		}

		for attName := range attSet {
			if physicalPath, exists := validAttachments[attName]; exists {
				fullRecord.Input.Attachments = append(fullRecord.Input.Attachments, &pb.Attachment{
					FileName:       attName,
					LocalVaultPath: physicalPath,
				})
			}
		}

		// 4. Create the Cognitive Record (Strip code blocks > 10 lines)
		cogRecord := proto.Clone(fullRecord).(*pb.AiRecord)
		cogRecord.Input.Narrative = stripLargeCodeBlocks(cogRecord.Input.Narrative)
		cogRecord.Output.Narrative = stripLargeCodeBlocks(cogRecord.Output.Narrative)

		// 5. Write to both ledger trees
		fullPath := filepath.Join(sourceVault, "ai-records", "full", year, month)
		cogPath := filepath.Join(sourceVault, "ai-records", "cognitive", year, month)

		WriteRecord(fullPath, filename, fullRecord)
		WriteRecord(cogPath, filename, cogRecord)

		count++
	}

	fmt.Printf("[✓] Extracted %d Gemini interactions safely to full and cognitive ledgers.\n", count)
}

// stripLargeCodeBlocks safely truncates ``` blocks longer than 10 lines
func stripLargeCodeBlocks(text string) string {
	re := regexp.MustCompile("(?s)```(.*?)```")
	return re.ReplaceAllStringFunc(text, func(match string) string {
		lines := strings.Split(match, "\n")
		// > 12 because the enclosing backticks count as lines
		if len(lines) > 12 {
			return "```\n[Code block > 10 lines stripped for cognitive view]\n```"
		}
		return match
	})
}
