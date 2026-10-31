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

package ggo

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/peterjbraam/ggo/internal/core"
	"github.com/peterjbraam/ggo/web"
	"github.com/yuin/goldmark"
)

type AgentEvent struct {
	Type    string
	Payload string
}

type WebServer struct {
	tmpl    *template.Template
	clients map[chan AgentEvent]bool
	mu      sync.Mutex
	md      goldmark.Markdown
}

func NewWebServer() *WebServer {
	tmpl, _ := template.ParseFS(web.FS, "templates/*.html")
	return &WebServer{
		tmpl:    tmpl,
		clients: make(map[chan AgentEvent]bool),
		md:      goldmark.New(),
	}
}

func (ws *WebServer) Start(port string) {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		fmt.Printf("[Web] Warning: Could not map static folder: %v\n", err)
	} else {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/api/context", ws.handleAttachContext)
	mux.HandleFunc("/api/prompt", ws.handlePrompt)
	mux.HandleFunc("/api/stream", ws.handleSSE)

	fmt.Printf("[Web] Interface running at http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Printf("[Web] Server crashed: %v\n", err)
	}
}

func (ws *WebServer) broadcast(event AgentEvent) {
	fmt.Printf("[DEBUG-TRACE] [WEB] Acquiring mutex to broadcast event: %s\n", event.Type)
	ws.mu.Lock()
	defer ws.mu.Unlock()

	clientCount := len(ws.clients)
	fmt.Printf("[DEBUG-TRACE] [WEB] Broadcasting to %d connected clients\n", clientCount)

	for clientChan := range ws.clients {
		select {
		case clientChan <- event:
			fmt.Printf("[DEBUG-TRACE] [WEB] Successfully pushed to client channel\n")
		default:
			fmt.Printf("[DEBUG-TRACE] [WEB] WARNING: Client channel full/blocked, dropping event\n")
		}
	}
}

func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"ActiveProj": core.ProjectCfg.ProjectName,
		"ActiveChat": core.GetActiveChat(),
		"Included":   core.ProjectSt.IncludedData,
	}
	ws.tmpl.ExecuteTemplate(w, "index.html", data)
}

func (ws *WebServer) handleAttachContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}

	targetPath := r.FormValue("filepath")
	if targetPath != "" {
		handleContextAdd(r.Context(), []string{targetPath})
	}

	ws.tmpl.ExecuteTemplate(w, "context-list.html", core.ProjectSt.IncludedData)
}

func (ws *WebServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[DEBUG-TRACE] [SSE] New browser connection attempt\n")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		fmt.Printf("[DEBUG-TRACE] [SSE] ERROR: Client does not support flushing\n")
		return
	}

	clientChan := make(chan AgentEvent, 10)
	ws.mu.Lock()
	ws.clients[clientChan] = true
	ws.mu.Unlock()
	fmt.Printf("[DEBUG-TRACE] [SSE] Client registered successfully\n")

	defer func() {
		fmt.Printf("[DEBUG-TRACE] [SSE] Client disconnected, cleaning up\n")
		ws.mu.Lock()
		delete(ws.clients, clientChan)
		close(clientChan)
		ws.mu.Unlock()
	}()

	for {
		select {
		case <-r.Context().Done():
			fmt.Printf("[DEBUG-TRACE] [SSE] HTTP Request context cancelled by browser\n")
			return
		case event := <-clientChan:
			fmt.Printf("[DEBUG-TRACE] [SSE] Received event %s on channel. Flushing to browser...\n", event.Type)

			// STRICT SSE MULTILINE PROTOCOL:
			// Each line of the payload MUST be prefixed with "data: "
			fmt.Fprintf(w, "event: %s\n", event.Type)
			lines := strings.Split(event.Payload, "\n")
			for _, line := range lines {
				fmt.Fprintf(w, "data: %s\n", line)
			}
			fmt.Fprintf(w, "\n\n")
			flusher.Flush()
			fmt.Printf("[DEBUG-TRACE] [SSE] Flush complete\n")
		}
	}
}

func (ws *WebServer) handlePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	promptText := r.FormValue("prompt")
	if strings.TrimSpace(promptText) == "" {
		return
	}

	fmt.Printf("[DEBUG-TRACE] [WEB] handlePrompt POST received: %q\n", promptText)

	// Inject the user card AND the thinking indicator into the chat-log
	fmt.Fprintf(w, `
		<div class="writing-card">
			<div class="meta">
				<span class="meta-date">User</span>
			</div>
			<div class="card-body">
				<pre><code>%s</code></pre>
			</div>
		</div>
		<div id="thinking-indicator" class="meta" style="margin-top: 2rem;">
			<span class="meta-date" style="color: #007acc;">Thinking...</span>
		</div>`, template.HTMLEscapeString(promptText))

	ws.broadcast(AgentEvent{Type: "status", Payload: "Dispatching to Gemini..."})

	fmt.Printf("[DEBUG-TRACE] [WEB] Spawning runRealAI goroutine...\n")
	go ws.runRealAI(context.Background(), promptText)
}

func (ws *WebServer) runRealAI(ctx context.Context, prompt string) {
	fmt.Printf("[DEBUG-TRACE] [WEB] runRealAI goroutine started. Calling DispatchPrompt...\n")

	response, err := DispatchPrompt(ctx, prompt, "ggo-web")

	fmt.Printf("[DEBUG-TRACE] [WEB] DispatchPrompt returned. Err: %v\n", err)

	if err != nil {
		ws.broadcast(AgentEvent{Type: "message", Payload: fmt.Sprintf(`
			<div hx-swap-oob="outerHTML:#thinking-indicator"></div>
			<div class="meta"><span class="meta-date" style="color:red;">API Error: %v</span></div>`, err)})
		return
	}

	fmt.Printf("[DEBUG-TRACE] [WEB] Parsing %d bytes of Markdown to HTML...\n", len(response))
	var buf bytes.Buffer
	if err := ws.md.Convert([]byte(response), &buf); err != nil {
		fmt.Printf("[DEBUG-TRACE] [WEB] Markdown parse failed: %v\n", err)
		ws.broadcast(AgentEvent{Type: "message", Payload: `
			<div hx-swap-oob="outerHTML:#thinking-indicator"></div>
			<div class="meta"><span class="meta-date" style="color:red;">Markdown parsing error.</span></div>`})
		return
	}

	finalHTML := fmt.Sprintf(`
		<div hx-swap-oob="outerHTML:#thinking-indicator"></div>
		<div class="writing-card">
			<div class="meta">
				<span class="meta-date">Daemon Response</span>
			</div>
			<div class="card-body">
				%s
			</div>
		</div>`, buf.String())

	fmt.Printf("[DEBUG-TRACE] [WEB] HTML rendered. Calling broadcast()...\n")
	ws.broadcast(AgentEvent{Type: "message", Payload: finalHTML})
	ws.broadcast(AgentEvent{Type: "status", Payload: ""})

	if ResponsePipe != nil {
		fmt.Printf("[DEBUG-TRACE] [WEB] Writing to Emacs ResponsePipe...\n")
		ResponsePipe.WriteString(response + "\n")
	}
	fmt.Printf("[DEBUG-TRACE] [WEB] runRealAI goroutine complete.\n")
}
