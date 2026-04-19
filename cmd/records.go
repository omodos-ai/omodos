package cmd

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

// ReadRecord safely loads a JSON file into the strict Protobuf struct.
// protojson automatically handles unknown fields and schema evolution safely.
func ReadRecord(path string) (*pb.AiRecord, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	record := &pb.AiRecord{}

	// Unmarshal using protojson (DiscardUnknown allows reading older/newer schemas safely)
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	if err := unmarshaler.Unmarshal(bytes, record); err != nil {
		return nil, fmt.Errorf("failed to parse AiRecord protobuf from JSON: %v", err)
	}

	// Future-proofing: Reject records from a vastly future version of the omo/ggo binary
	if record.Version > CurrentSchemaVersion {
		return nil, fmt.Errorf("record version %d is newer than current CLI version %d", record.Version, CurrentSchemaVersion)
	}

	return record, nil
}

// WriteRecord ensures strict Protobuf serialization to human-readable JSON on disk
func WriteRecord(targetDir, filename string, record *pb.AiRecord) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(targetDir, filename)

	record.Version = CurrentSchemaVersion

	// Serialize Protobuf to beautifully indented JSON
	marshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false, // Keeps the JSON lean by dropping empty arrays/strings
	}

	bytes, err := marshaler.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal AiRecord protobuf: %v", err)
	}

	return os.WriteFile(path, bytes, 0644)
}

// MakeSafeSlug generates a predictable, safe filename prefix from prompt text
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
