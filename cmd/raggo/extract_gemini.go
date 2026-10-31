// Copyright 2025-2026 Peter Braam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raggo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/internal/core"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/peterjbraam/ggo/pb/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func RunGeminiExtract(recordsDir, exportDir string, overwrite bool) {
	validAttachments := make(map[string]string)
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

	wrappedJSON := append([]byte(`{"activities": `), rawBytes...)
	wrappedJSON = append(wrappedJSON, []byte(`}`)...)

	var export pb.GeminiTakeoutExport

	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: false}
	if err := unmarshaler.Unmarshal(wrappedJSON, &export); err != nil {
		fmt.Printf("\n[FATAL STRICT PARSE ERROR]\n")
		fmt.Printf("Google Takeout schema has changed. An undocumented JSON field was detected.\n")
		fmt.Printf("System halted to prevent data corruption.\n")
		fmt.Printf("-> ACTION REQUIRED: Call omodos.ai to get a fix.\n\n")
		fmt.Printf("Raw Error: %v\n", err)
		os.Exit(1)
	}

	activities := export.Activities
	for i, j := 0, len(activities)-1; i < j; i, j = i+1, j-1 {
		activities[i], activities[j] = activities[j], activities[i]
	}

	converter := md.NewConverter("", true, nil)
	count := 0
	skipped := 0

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
		year := fmt.Sprintf("%04d", promptTime.Year())
		month := fmt.Sprintf("%02d", promptTime.Month())
		timestamp := promptTime.Format("20060102T150405Z")
		safeSubject := core.MakeSafeSlug(prompt)
		filename := fmt.Sprintf("%s-%s.json", timestamp, safeSubject)

		targetDir := filepath.Join(recordsDir, year, month)
		fullPath := filepath.Join(targetDir, filename)

		if !overwrite {
			if _, err := os.Stat(fullPath); err == nil {
				skipped++
				continue
			}
		}

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
				record.Input.Attachments = append(record.Input.Attachments, &pb.Attachment{
					FileName:       attName,
					LocalVaultPath: physicalPath,
				})
			}
		}

		core.WriteRecord(targetDir, filename, record)
		count++
	}

	fmt.Printf("[✓] Extracted %d Gemini interactions safely to JSON ledger (Skipped: %d).\n", count, skipped)
}
