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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

// Helper to keep this test completely standalone
func expandTestTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// The generic backoff wrapper we want to test for the final ggo implementation
func withRetry(t *testing.T, maxRetries int, operation func() error) error {
	for i := 0; i <= maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		errStr := strings.ToLower(err.Error())
		// Catch 503 (Unavailable), 429 (Resource Exhausted), 500 (Internal), 504 (Gateway Timeout)
		isTransient := strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "504") ||
			strings.Contains(errStr, "unavailable") ||
			strings.Contains(errStr, "exhausted") ||
			strings.Contains(errStr, "quota")

		if isTransient {
			if i < maxRetries {
				backoff := time.Duration(1<<i) * time.Second // 1s, 2s, 4s exponential backoff
				t.Logf("    [Retry %d/%d] Transient error caught (%v). Backing off for %v...", i+1, maxRetries, err, backoff)
				time.Sleep(backoff)
				continue
			}
		} else {
			// If it's a hard error (like 401 Unauthorized or 400 Bad Request), fail immediately
			t.Logf("    [Fatal] Non-transient error detected: %v", err)
			return err
		}
		return err // Return if max retries exceeded
	}
	return nil
}

func TestStandaloneAPIStress(t *testing.T) {
	ctx := context.Background()

	t.Log("=======================================================")
	t.Log(" PHASE 1: Key Validation")
	t.Log("=======================================================")

	apiKey := os.Getenv("GEMINI_API_KEY")
	keySource := "Environment Variable (GEMINI_API_KEY)"

	if apiKey == "" {
		keyPath := expandTestTilde("~/secrets/gemini-token")
		if keyBytes, err := os.ReadFile(keyPath); err == nil {
			apiKey = strings.TrimSpace(string(keyBytes))
			keySource = keyPath
		}
	}

	if apiKey == "" {
		t.Fatal("FAILED: No API key found in environment or ~/secrets/gemini-token")
	}

	// Print masked key to verify which one is actually being loaded
	maskedKey := fmt.Sprintf("%s...%s", apiKey[:6], apiKey[len(apiKey)-4:])
	t.Logf("Loaded Key : %s", maskedKey)
	t.Logf("Source     : %s", keySource)
	t.Logf("Length     : %d chars", len(apiKey))

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		t.Fatalf("FAILED to initialize genai SDK: %v", err)
	}

	t.Log("=======================================================")
	t.Log(" PHASE 2 & 3: Burst Profiling & Backoff Recovery")
	t.Log("=======================================================")

	// Testing across the cutting-edge lineup
	models := []string{
		"gemini-3.1-pro-preview",
		"gemini-2.5-pro",
		"gemini-3.1-flash-lite",
	}

	for _, modelName := range models {
		t.Logf("\n--- Testing Model: %s ---", modelName)

		burstCount := 5 // Fire 5 rapid requests to try and induce a load-balancer shed

		// Test A: No Retries (To prove where and how it fails naturally)
		t.Logf("  [Test A] Firing %d rapid requests (NO RETRIES)...", burstCount)
		for i := 1; i <= burstCount; i++ {
			prompt := fmt.Sprintf("Burst test A, request %d. Reply with a single word: Received.", i)
			start := time.Now()

			_, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), nil)
			duration := time.Since(start)

			if err != nil {
				t.Logf("    Request %d: \033[31mFAILED\033[0m in %v - %v", i, duration, err)
				break // Stop the burst on failure to keep the log clean
			} else {
				t.Logf("    Request %d: \033[32mOK\033[0m (%v)", i, duration)
			}
		}

		// Brief pause between tests
		time.Sleep(2 * time.Second)

		// Test B: With Retries (To prove the backoff logic rescues the connection)
		t.Logf("  [Test B] Firing %d rapid requests (WITH RETRIES)...", burstCount)
		for i := 1; i <= burstCount; i++ {
			prompt := fmt.Sprintf("Burst test B, request %d. Reply with a single word: Received.", i)
			start := time.Now()

			err := withRetry(t, 3, func() error {
				_, sendErr := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), nil)
				return sendErr
			})
			duration := time.Since(start)

			if err != nil {
				t.Logf("    Request %d: \033[31mFATAL FAILURE\033[0m in %v - %v", i, duration, err)
			} else {
				t.Logf("    Request %d: \033[32mSUCCESS\033[0m (%v)", i, duration)
			}
		}
	}

	t.Log("=======================================================")
	t.Log(" DIAGNOSTICS COMPLETE")
	t.Log("=======================================================")
}
