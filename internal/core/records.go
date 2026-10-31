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

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/peterjbraam/ggo/pb/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const CurrentSchemaVersion = 1

var WarnedSchemaVersion bool

func ReadRecord(path string) (*pb.AiRecord, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	record := &pb.AiRecord{}

	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	if err := unmarshaler.Unmarshal(bytes, record); err != nil {
		return nil, fmt.Errorf("failed to parse AiRecord protobuf from JSON: %v", err)
	}

	if record.Version > CurrentSchemaVersion {
		if !WarnedSchemaVersion {
			fmt.Printf("\n\033[31m[WARNING] Found records with version %d (newer than current CLI version %d). Skipping to prevent data corruption. Please upgrade omodos.\033[0m\n\n", record.Version, CurrentSchemaVersion)
			WarnedSchemaVersion = true
		}
		return nil, fmt.Errorf("record version %d is newer than current CLI version %d", record.Version, CurrentSchemaVersion)
	}

	return record, nil
}

func WriteRecord(targetDir, filename string, record *pb.AiRecord) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(targetDir, filename)

	record.Version = CurrentSchemaVersion

	marshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
	}

	bytes, err := marshaler.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal AiRecord protobuf: %v", err)
	}

	return os.WriteFile(path, bytes, 0644)
}

func MakeSafeSlug(str string) string {
	words := strings.Fields(str)
	limit := 5
	if len(words) < limit {
		limit = len(words)
	}
	subjectStr := strings.Join(words[:limit], " ")
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := reg.ReplaceAllString(subjectStr, "-")
	slug = strings.ToLower(strings.Trim(slug, "-"))
	if len(slug) > 20 {
		slug = strings.TrimRight(slug[:20], "-")
	}
	if slug == "" {
		slug = "interaction"
	}
	return slug
}
