package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const fakeOllamaVersion = "0.12.5"

type modelSpec struct {
	contextLength int
	capabilities  []string
}

// The fixed model listing (see CONTEXT.md). Names pass through to DeepSeek.
var models = map[string]modelSpec{
	"deepseek-chat":     {contextLength: 131072, capabilities: []string{"completion"}},
	"deepseek-reasoner": {contextLength: 131072, capabilities: []string{"completion", "thinking"}},
}

var modelDetails = map[string]any{
	"parent_model":       "",
	"format":             "gguf",
	"family":             "deepseek",
	"families":           []string{"deepseek"},
	"parameter_size":     "671B",
	"quantization_level": "F16",
}

type server struct {
	cfg    config
	client *http.Client
}

func newServer(cfg config) *server {
	return &server{cfg: cfg, client: &http.Client{}}
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("POST /api/show", s.handleShow)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "Ollama is running")
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": fakeOllamaVersion})
}

func (s *server) handleTags(w http.ResponseWriter, _ *http.Request) {
	list := make([]map[string]any, 0, len(models))
	for _, name := range []string{"deepseek-chat", "deepseek-reasoner"} {
		list = append(list, map[string]any{
			"name":        name,
			"model":       name,
			"modified_at": "2026-01-01T00:00:00Z",
			"size":        int64(12000000000),
			"digest":      "abcde12345fghij67890klmno1234567890abcdef",
			"details":     modelDetails,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": list})
}

func (s *server) handleShow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
		Name  string `json:"name"` // legacy field older clients send
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := req.Model
	if name == "" {
		name = req.Name
	}
	spec, ok := models[name]
	if !ok {
		writeError(w, http.StatusNotFound, "model %q not found", name)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"modelfile":  "",
		"parameters": "",
		"template":   "",
		"details":    modelDetails,
		"model_info": map[string]any{
			"general.architecture":    "deepseek",
			"deepseek.context_length": spec.contextLength,
			"general.parameter_count": int64(671000000000),
		},
		"capabilities": spec.capabilities,
	})
}

func (s *server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ollamaChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" || len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "model and messages are required")
		return
	}
	stream := req.Stream == nil || *req.Stream // Ollama defaults to streaming

	body, err := json.Marshal(toDeepSeekRequest(req, stream))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding upstream request: %v", err)
		return
	}
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		s.cfg.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "building upstream request: %v", err)
		return
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+s.cfg.apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.relayUpstreamError(w, resp)
		return
	}

	if stream {
		s.streamChat(w, r, resp, req.Model)
	} else {
		s.completeChat(w, resp, req.Model)
	}
}

// relayUpstreamError propagates DeepSeek's status code and error message in
// Ollama's error shape, so clients see the real cause (bad key, rate limit).
func (s *server) relayUpstreamError(w http.ResponseWriter, resp *http.Response) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	msg := string(body)
	var dsErr dsErrorResponse
	if err := json.Unmarshal(body, &dsErr); err == nil && dsErr.Error.Message != "" {
		msg = dsErr.Error.Message
	}
	log.Printf("upstream error %d: %s", resp.StatusCode, msg)
	writeError(w, resp.StatusCode, "%s", msg)
}

func (s *server) completeChat(w http.ResponseWriter, resp *http.Response, model string) {
	start := time.Now()
	var ds dsChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		writeError(w, http.StatusBadGateway, "decoding upstream response: %v", err)
		return
	}
	if len(ds.Choices) == 0 {
		writeError(w, http.StatusBadGateway, "upstream returned no choices")
		return
	}
	choice := ds.Choices[0]
	if ds.Model != "" {
		model = ds.Model
	}
	out := ollamaChatResponse{
		Model:     model,
		CreatedAt: time.Now().UTC().Format(timeFormat),
		Message: ollamaMessage{
			Role:     "assistant",
			Content:  choice.Message.Content,
			Thinking: choice.Message.ReasoningContent,
		},
		Done:          true,
		DoneReason:    choice.FinishReason,
		TotalDuration: time.Since(start).Nanoseconds(),
	}
	if out.DoneReason == "" {
		out.DoneReason = "stop"
	}
	if ds.Usage != nil {
		out.EvalCount = ds.Usage.CompletionTokens
		out.PromptEvalCount = ds.Usage.PromptTokens
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) streamChat(w http.ResponseWriter, r *http.Request, resp *http.Response, model string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)

	tr := newStreamTranslator(model)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		for _, out := range tr.translate(scanner.Bytes()) {
			if err := enc.Encode(out); err != nil {
				return // client went away
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
	if err := scanner.Err(); err != nil && r.Context().Err() == nil {
		log.Printf("upstream stream error: %v", err)
		_ = enc.Encode(map[string]string{"error": fmt.Sprintf("upstream stream error: %v", err)})
	} else if !tr.emittedFin {
		// Upstream closed without [DONE]; still hand the client a final chunk.
		for _, out := range tr.finish(nil) {
			_ = enc.Encode(out)
		}
	}
	if flusher != nil {
		flusher.Flush()
	}
}
