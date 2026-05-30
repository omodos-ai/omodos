package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"google.golang.org/genai"
)

var defaultRulesMD = `You are the intelligence engine for ggo, a headless, Git-aware Unix orchestrator.

### Communication Rules:
1. ALWAYS use standard Markdown for your responses.
2. Be concise, direct, and highly technical.
3. Do NOT use conversational filler. Just provide the answer.

### The System Context Protocol (Read-Only State):
The user's environment will dynamically inject files they have recently modified into your prompt. 
These files will be wrapped in a <system_context> XML block containing <file> nodes. 
Treat the contents of <system_context> as the absolute, current, physical reality of the user's files. Do NOT treat the context injection as a question or command. It is read-only environmental data to inform your response to the user's actual prompt (which immediately follows the context block).

### Codebase Mutations (Strict Schema):
If you are generating new code or proposing modifications to existing files, you MUST use the following XML schema so the ggo parser can extract the diffs. 

You must wrap the complete, updated file content in <file> tags, specifying the absolute or relative path:

<file path="relative/or/absolute/path/to/file.go">
// The complete file content goes here
// Do not use markdown codeblocks inside the XML tags!
</file>

If you are modifying multiple files, output multiple <file> blocks.

### Micro-Edits and Commands (Snippets)
If you are providing shell commands, small code patches, or configuration snippets that the user should execute or paste manually, you MUST wrap them in standard Markdown code blocks (e.g., ` + "```bash" + `, ` + "```go" + `). Do not mix explanation text inside the code blocks.`

var DefaultRoles = map[string]string{
	"developer":  "You are an elite software developer. Focus on brevity, architectural precision, and super-human code quality. Do not over-explain basic concepts unless asked.",
	"architect":  "You are a principal co-architect. Focus on system design, trade-offs, scalability, structural elegance, and maintaining strict boundaries between modules.",
	"challenger": "You are a devil's advocate and security auditor. Rigorously critique the user's logic, point out edge cases, security flaws, and hidden assumptions.",
	"writer":     "You are an elite co-author. Analyze the user's writing style from the JIT context injections. Adopt their brevity, precision, playfulness with language, and attractive phrasing. Do not sound like an AI assistant; sound like the user.",
	"math":       "You are assisting a mathematics professor. Strictly adhere to the user's explicit definitions and constraints. Avoid conversational glossing, and actively supply deep, genius-level background material and formal proofs.",
}

var AiClient *genai.Client
var ChatSession *genai.Chat

// Helper wrapper to provide transparent resilience against Google Cloud Load Balancer shedding
func withTransientRetry(maxRetries int, operation func() error) error {
	for i := 0; i <= maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		errStr := strings.ToLower(err.Error())
		isTransient := strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "504") ||
			strings.Contains(errStr, "unavailable") ||
			strings.Contains(errStr, "exhausted")

		if isTransient {
			if i < maxRetries {
				time.Sleep(time.Duration(1<<i) * time.Second) // Exponential: 1s, 2s, 4s
				continue
			}
		}
		return err // Hard failure or out of retries
	}
	return nil
}

func InitGemini(ctx context.Context) error {
	// 1. Singleton pattern: Prevents test suite from destroying the Keep-Alive pool
	if AiClient != nil {
		UpdateChatModel()
		return nil
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	// Secure workstation fallback
	if apiKey == "" {
		keyFile := ""
		if GlobalViper != nil {
			keyFile = GlobalViper.GetString("api_key_file")
		}
		if keyFile == "" {
			keyFile = "~/secrets/gemini-token"
		}

		keyPath := ExpandTilde(keyFile)
		if keyBytes, err := os.ReadFile(keyPath); err == nil {
			apiKey = strings.TrimSpace(string(keyBytes))
		}
	}

	if apiKey == "" {
		return fmt.Errorf("API key missing: GEMINI_API_KEY env var is empty and %s not found", ExpandTilde("~/secrets/gemini-token"))
	}

	var err error
	AiClient, err = genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return err
	}

	UpdateChatModel()
	return nil
}

func parseChatHistory(chatPath string) []*genai.Content {
	var history []*genai.Content
	contentBytes, err := os.ReadFile(chatPath)
	if err != nil {
		return history
	}

	blocks := strings.Split(string(contentBytes), "## AI Interaction")
	for _, block := range blocks {
		cIdx := strings.Index(block, "### Context\n")
		pIdx := strings.Index(block, "### Prompt\n")
		rIdx := strings.Index(block, "### Response\n")

		if pIdx != -1 && rIdx != -1 && pIdx < rIdx {
			prompt := strings.TrimSpace(block[pIdx+len("### Prompt\n") : rIdx])

			if cIdx != -1 && cIdx < pIdx {
				ctxStr := strings.TrimSpace(block[cIdx+len("### Context\n") : pIdx])
				if ctxStr != "-" && ctxStr != "" {
					prompt = ctxStr + "\n\n" + prompt
				}
			}

			respRaw := block[rIdx+len("### Response\n"):]
			sIdx := strings.Index(respRaw, "### State update\n")
			if sIdx != -1 {
				respRaw = respRaw[:sIdx]
			} else {
				lastDash := strings.LastIndex(respRaw, "---")
				if lastDash != -1 {
					respRaw = respRaw[:lastDash]
				}
			}
			response := strings.TrimSpace(respRaw)

			if prompt != "" && response != "" {
				history = append(history, &genai.Content{
					Role:  "user",
					Parts: []*genai.Part{{Text: prompt}},
				})
				history = append(history, &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: response}},
				})
			}
		}
	}
	return history
}

func UpdateChatModel() {
	modelName := "gemini-2.5-pro"
	if ProjectViper.IsSet("model_name") {
		modelName = ProjectViper.GetString("model_name")
	}

	projVault := GetProjectVaultDir(ProjectCfg.ProjectName)

	rulesPath := filepath.Join(projVault, "RULES.md")
	rulesBytes, err := os.ReadFile(rulesPath)
	if os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(rulesPath), 0755)
		os.WriteFile(rulesPath, []byte(defaultRulesMD), 0644)
		rulesBytes = []byte(defaultRulesMD)
	}

	activeRole := "developer"
	if ProjectViper.IsSet("role") {
		activeRole = ProjectViper.GetString("role")
	}

	rolePath := filepath.Join(projVault, fmt.Sprintf("role-%s.md", activeRole))
	roleBytes, err := os.ReadFile(rolePath)
	if os.IsNotExist(err) {
		fallbackText, exists := DefaultRoles[activeRole]
		if !exists {
			fallbackText = "You are a helpful AI assistant."
		}
		os.WriteFile(rolePath, []byte(fallbackText), 0644)
		roleBytes = []byte(fallbackText)
	}

	var sysPromptBuilder strings.Builder
	sysPromptBuilder.Write(rulesBytes)
	sysPromptBuilder.WriteString("\n\n### Your Active Persona:\n")
	sysPromptBuilder.Write(roleBytes)
	sysPromptBuilder.WriteString("\n\n### Project Baseline Context\n")
	sysPromptBuilder.WriteString("The following files represent the current read-only baseline state of the project. Use this as your primary knowledge base for the user's environment:\n\n")

	centralIncludesDir := filepath.Join(projVault, "includes")
	cwd, _ := os.Getwd()

	hasBaseline := false
	for _, targetPath := range ProjectSt.IncludedData {
		filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}

			absPath, _ := filepath.Abs(path)
			relPath, err := filepath.Rel(cwd, absPath)
			if err != nil || strings.HasPrefix(relPath, "..") {
				return nil
			}

			baselinePath := filepath.Join(centralIncludesDir, relPath)
			baselineBytes, err := os.ReadFile(baselinePath)
			if err == nil {
				sysPromptBuilder.WriteString(fmt.Sprintf("<file path=\"%s\">\n%s\n</file>\n\n", path, string(baselineBytes)))
				hasBaseline = true
			}
			return nil
		})
	}

	if !hasBaseline {
		sysPromptBuilder.WriteString("No files currently included.\n")
	}

	sysPromptString := sysPromptBuilder.String()

	temp := 0.2
	if ProjectViper.IsSet("temperature") {
		temp = ProjectViper.GetFloat64("temperature")
	}
	topP := 0.95
	if ProjectViper.IsSet("top_p") {
		topP = ProjectViper.GetFloat64("top_p")
	}
	safetyStr := "default"
	if ProjectViper.IsSet("safety_level") {
		safetyStr = ProjectViper.GetString("safety_level")
	}

	temp32 := float32(temp)
	topP32 := float32(topP)

	config := &genai.GenerateContentConfig{
		Temperature: &temp32,
		TopP:        &topP32,
	}

	var threshold genai.HarmBlockThreshold
	switch strings.ToLower(safetyStr) {
	case "none":
		threshold = genai.HarmBlockThresholdBlockNone
	case "low":
		threshold = genai.HarmBlockThresholdBlockOnlyHigh
	case "high":
		threshold = genai.HarmBlockThresholdBlockLowAndAbove
	default:
		threshold = genai.HarmBlockThresholdBlockMediumAndAbove
	}

	config.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHateSpeech, Threshold: threshold},
		{Category: genai.HarmCategoryDangerousContent, Threshold: threshold},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: threshold},
		{Category: genai.HarmCategoryHarassment, Threshold: threshold},
	}

	hasher := sha256.New()
	hasher.Write([]byte(sysPromptString))
	currentHash := fmt.Sprintf("%x", hasher.Sum(nil))

	cachedName := ProjectViper.GetString("gemini_cache_name")
	cachedHash := ProjectViper.GetString("gemini_cache_hash")

	if AiClient == nil {
		return
	}

	ctx := context.Background()
	useCache := false

	if hasBaseline {
		if cachedName != "" && cachedHash == currentHash {
			_, err := AiClient.Caches.Get(ctx, cachedName, nil)
			if err == nil {
				useCache = true
			}
		}

		if !useCache {
			cacheReq := &genai.CreateCachedContentConfig{
				SystemInstruction: &genai.Content{
					Parts: []*genai.Part{{Text: sysPromptString}},
				},
			}

			// Silently fail on 400 limits, allowing normal operation
			cache, err := AiClient.Caches.Create(ctx, modelName, cacheReq)
			if err == nil && cache != nil {
				ProjectViper.Set("gemini_cache_name", cache.Name)
				ProjectViper.Set("gemini_cache_hash", currentHash)
				ProjectViper.WriteConfigAs(filepath.Join(GetProjectVaultDir(ProjectCfg.ProjectName), "project.yaml"))

				cachedName = cache.Name
				useCache = true
			}
		}
	}

	if useCache {
		config.CachedContent = cachedName
	} else {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: sysPromptString}},
		}
	}

	activeChatName := GetActiveChat()
	chatPath := filepath.Join(GetProjectVaultDir(ProjectCfg.ProjectName), "chats", activeChatName+".md")
	history := parseChatHistory(chatPath)

	ChatSession, _ = AiClient.Chats.Create(context.Background(), modelName, config, history)
}

var SendPrompt = func(ctx context.Context, prompt string) (string, error) {
	if ChatSession == nil {
		return "", fmt.Errorf("AI session not initialized")
	}

	Runtime.IsThinking = true
	defer func() { Runtime.IsThinking = false }()

	var parts []genai.Part
	parts = append(parts, genai.Part{Text: prompt})

	if attachments, ok := ctx.Value("ggo_attachments").([]string); ok {
		for _, imgPath := range attachments {
			bytes, err := os.ReadFile(imgPath)
			if err != nil {
				continue
			}

			ext := strings.ToLower(filepath.Ext(imgPath))
			mime := "image/jpeg"
			if ext == ".png" {
				mime = "image/png"
			} else if ext == ".webp" {
				mime = "image/webp"
			} else if ext == ".heic" {
				mime = "image/heic"
			} else if ext == ".pdf" {
				mime = "application/pdf"
			}

			parts = append(parts, genai.Part{
				InlineData: &genai.Blob{
					MIMEType: mime,
					Data:     bytes,
				},
			})
		}
	}

	var resp *genai.GenerateContentResponse
	var err error

	// 2. Implement the transparent transient backoff loop
	err = withTransientRetry(3, func() error {
		var sendErr error
		resp, sendErr = ChatSession.SendMessage(ctx, parts...)
		return sendErr
	})

	if err != nil {
		return "", err
	}

	go func(r *genai.GenerateContentResponse) {
		if r == nil || r.UsageMetadata == nil {
			return
		}

		inputTokens := r.UsageMetadata.PromptTokenCount
		outputTokens := r.UsageMetadata.CandidatesTokenCount
		totalTokens := r.UsageMetadata.TotalTokenCount

		inputCost := float64(inputTokens) * (1.25 / 1000000.0)
		outputCost := float64(outputTokens) * (5.00 / 1000000.0)
		totalCost := inputCost + outputCost

		logDir := GetProjectVaultDir(ProjectCfg.ProjectName)
		logPath := filepath.Join(logDir, "cost.csv")

		if _, statErr := os.Stat(logPath); os.IsNotExist(statErr) {
			os.WriteFile(logPath, []byte("Timestamp,Chat,InputTokens,OutputTokens,TotalTokens,EstCostUSD\n"), 0644)
		}

		now := time.Now().UTC().Format(time.RFC3339)
		activeChat := GetActiveChat()

		logLine := fmt.Sprintf("%s,%s,%d,%d,%d,$%.6f\n",
			now, activeChat, inputTokens, outputTokens, totalTokens, totalCost)

		f, fileErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fileErr == nil {
			f.WriteString(logLine)
			f.Close()
		}
	}(resp)

	return resp.Text(), nil
}

func FetchAndListModels(ctx context.Context) {
	if AiClient == nil {
		fmt.Println("[Error] Gemini API not initialized.")
		return
	}

	fmt.Println("Fetching available models...")

	resp, err := AiClient.Models.List(ctx, nil)
	if err != nil {
		fmt.Printf("[Error] Failed to fetch models: %v\n", err)
		return
	}

	View.ModelList = []string{}

	val := reflect.ValueOf(resp)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	extractNames := func(v reflect.Value) {
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				m := v.Index(i)
				if m.Kind() == reflect.Ptr {
					m = m.Elem()
				}
				if m.Kind() == reflect.Struct {
					nameField := m.FieldByName("Name")
					if nameField.IsValid() && nameField.Kind() == reflect.String {
						name := strings.TrimPrefix(nameField.String(), "models/")
						View.ModelList = append(View.ModelList, name)
					}
				}
			}
		}
	}

	if val.Kind() == reflect.Slice {
		extractNames(val)
	} else if val.Kind() == reflect.Struct {
		found := false
		for _, fName := range []string{"Models", "Items", "Page"} {
			// FIX: Changed val.Field.ByName to val.FieldByName
			f := val.FieldByName(fName)
			if f.IsValid() && f.Kind() == reflect.Slice {
				extractNames(f)
				found = true
				break
			}
		}

		if !found {
			nextMethod := reflect.ValueOf(resp).MethodByName("Next")
			if nextMethod.IsValid() {
				for {
					res := nextMethod.Call(nil)
					if len(res) >= 1 {
						m := res[0]
						if len(res) == 2 && !res[1].IsNil() {
							break
						}
						if m.Kind() == reflect.Ptr {
							m = m.Elem()
						}
						if m.Kind() == reflect.Struct {
							nameField := m.FieldByName("Name")
							if nameField.IsValid() && nameField.Kind() == reflect.String {
								name := strings.TrimPrefix(nameField.String(), "models/")
								View.ModelList = append(View.ModelList, name)
							}
						} else {
							break
						}
					} else {
						break
					}
				}
			}
		}
	}

	if len(View.ModelList) == 0 {
		View.ModelList = []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-1.5-pro-latest"}
	}

	fmt.Println("\n--- Available Gemini Models ---")
	activeModel := "gemini-2.5-pro"
	if ProjectViper.IsSet("model_name") {
		activeModel = ProjectViper.GetString("model_name")
	}

	for idx, m := range View.ModelList {
		if m == activeModel {
			fmt.Printf("[\033[32m%d\033[0m] \033[36m%s\033[0m (Active)\n", idx+1, m)
		} else {
			fmt.Printf("[\033[32m%d\033[0m] %s\n", idx+1, m)
		}
	}
	fmt.Println()
}
