package raggo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/pb/v1"
)

func RunMdchatExtract(recordsDir, ingestDir string, overwrite bool) {
	totalWritten := 0
	totalSkipped := 0

	err := filepath.Walk(ingestDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		blocks := strings.Split(string(contentBytes), "## AI Interaction")
		for _, block := range blocks {
			if strings.TrimSpace(block) == "" {
				continue
			}

			dIdx := strings.Index(block, "### Date\n")
			cIdx := strings.Index(block, "### Chat\n")
			sIdx := strings.Index(block, "### Source\n")
			subjIdx := strings.Index(block, "### Subject\n")
			ctxIdx := strings.Index(block, "### Context\n")
			pIdx := strings.Index(block, "### Prompt\n")
			rIdx := strings.Index(block, "### Response\n")

			if dIdx == -1 || cIdx == -1 || pIdx == -1 || rIdx == -1 {
				continue
			}

			dateStr := strings.TrimSpace(block[dIdx+len("### Date\n") : cIdx])
			parsedTime, err := time.Parse(time.RFC3339, dateStr)
			if err != nil {
				continue
			}
			parsedTime = parsedTime.UTC()

			chatTitle := "unknown_chat"
			if sIdx != -1 && sIdx > cIdx {
				chatTitle = strings.TrimSpace(block[cIdx+len("### Chat\n") : sIdx])
			}

			modelName := "unknown_model"
			if sIdx != -1 {
				sourceLineEnd := strings.Index(block[sIdx:], "\n###")
				if sourceLineEnd != -1 {
					sourceLine := block[sIdx+len("### Source\n") : sIdx+sourceLineEnd]
					parts := strings.Split(sourceLine, "/")
					if len(parts) > 1 {
						modelName = strings.TrimSpace(parts[1])
					}
				}
			}

			promptStr := ""
			if pIdx != -1 && rIdx > pIdx {
				promptStr = strings.TrimSpace(block[pIdx+len("### Prompt\n") : rIdx])
			}

			respRaw := block[rIdx+len("### Response\n"):]
			stateUpdateIdx := strings.Index(respRaw, "### State update\n")
			if stateUpdateIdx != -1 {
				respRaw = respRaw[:stateUpdateIdx]
			} else {
				lastDash := strings.LastIndex(respRaw, "---")
				if lastDash != -1 {
					respRaw = respRaw[:lastDash]
				}
			}
			responseStr := strings.TrimSpace(respRaw)

			if promptStr == "" || responseStr == "" {
				continue
			}

			year := fmt.Sprintf("%04d", parsedTime.Year())
			month := fmt.Sprintf("%02d", parsedTime.Month())
			timestamp := parsedTime.Format("20060102T150405Z")
			safeSubject := core.MakeSafeSlug(promptStr)

			if subjIdx != -1 && ctxIdx != -1 && ctxIdx > subjIdx {
				subjRaw := strings.TrimSpace(block[subjIdx+len("### Subject\n") : ctxIdx])
				reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
				slug := reg.ReplaceAllString(subjRaw, "_")
				slug = strings.Trim(slug, "_")
				if len(slug) > 20 {
					slug = strings.TrimRight(slug[:20], "_")
				}
				if slug != "" {
					safeSubject = slug
				}
			}

			filename := fmt.Sprintf("%s-%s.json", timestamp, safeSubject)
			targetDir := filepath.Join(recordsDir, year, month)
			fullPath := filepath.Join(targetDir, filename)

			if !overwrite {
				if _, err := os.Stat(fullPath); err == nil {
					totalSkipped++
					continue
				}
			}

			record := &pb.AiRecord{
				Metadata: &pb.Metadata{
					TimestampUtc: parsedTime.Format(time.RFC3339),
					ChatId:       chatTitle,
					ModelName:    modelName,
				},
				Input: &pb.UserInput{
					Narrative: promptStr,
				},
				Output: &pb.AIResponse{
					Narrative: responseStr,
				},
			}

			core.WriteRecord(targetDir, filename, record)
			totalWritten++
		}
		return nil
	})

	if err != nil {
		fmt.Printf("[Error] Failed scanning mdchat directory: %v\n", err)
	}

	fmt.Printf("[✓] Extracted %d local markdown chat interactions to JSON ledger (Skipped: %d).\n", totalWritten, totalSkipped)
}
