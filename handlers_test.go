package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer wires the facade to a fake DeepSeek upstream.
func newTestServer(t *testing.T, upstream http.HandlerFunc) *httptest.Server {
	t.Helper()
	up := httptest.NewServer(upstream)
	t.Cleanup(up.Close)
	s := newServer(config{apiKey: "test-key", baseURL: up.URL})
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return ts
}

func TestRootAndVersion(t *testing.T) {
	ts := newTestServer(t, nil)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf [32]byte
	n, _ := resp.Body.Read(buf[:])
	if got := string(buf[:n]); got != "Ollama is running" {
		t.Errorf("root: got %q", got)
	}

	resp, err = http.Get(ts.URL + "/api/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.Version == "" {
		t.Errorf("version: %v, %+v", err, v)
	}
}

func TestTags(t *testing.T) {
	ts := newTestServer(t, nil)
	resp, err := http.Get(ts.URL + "/api/tags")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Models) != 2 || out.Models[0].Name != "deepseek-chat" || out.Models[1].Name != "deepseek-reasoner" {
		t.Errorf("unexpected models: %+v", out.Models)
	}
}

func TestShow(t *testing.T) {
	ts := newTestServer(t, nil)

	resp, err := http.Post(ts.URL+"/api/show", "application/json",
		strings.NewReader(`{"model":"deepseek-reasoner"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Capabilities) != 2 || out.Capabilities[1] != "thinking" {
		t.Errorf("reasoner capabilities: %v", out.Capabilities)
	}

	resp, err = http.Post(ts.URL+"/api/show", "application/json",
		strings.NewReader(`{"model":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown model: got status %d, want 404", resp.StatusCode)
	}
}

func TestChatNonStreaming(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header: %q", got)
		}
		var req dsChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding upstream request: %v", err)
		}
		if req.Stream {
			t.Error("expected non-streaming upstream request")
		}
		fmt.Fprint(w, `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
	})

	resp, err := http.Post(ts.URL+"/api/chat", "application/json",
		strings.NewReader(`{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"stream":false}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Message.Content != "Hi there" || !out.Done || out.DoneReason != "stop" ||
		out.EvalCount != 4 || out.PromptEvalCount != 12 {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestChatStreaming(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"model":"deepseek-reasoner","choices":[{"delta":{"reasoning_content":"thinking..."},"finish_reason":null}]}`,
		``,
		`data: {"model":"deepseek-reasoner","choices":[{"delta":{"content":"42"},"finish_reason":null}]}`,
		``,
		`data: {"model":"deepseek-reasoner","choices":[{"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: {"model":"deepseek-reasoner","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":2}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	})

	// stream omitted: must default to streaming like real Ollama
	resp, err := http.Post(ts.URL+"/api/chat", "application/json",
		strings.NewReader(`{"model":"deepseek-reasoner","messages":[{"role":"user","content":"?"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content type: %q", ct)
	}

	var lines []ollamaChatResponse
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var line ollamaChatResponse
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("bad NDJSON line %q: %v", scanner.Text(), err)
		}
		lines = append(lines, line)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Message.Thinking != "thinking..." || lines[0].Message.Content != "" {
		t.Errorf("thinking chunk: %+v", lines[0])
	}
	if lines[1].Message.Content != "42" {
		t.Errorf("content chunk: %+v", lines[1])
	}
	fin := lines[2]
	if !fin.Done || fin.DoneReason != "stop" || fin.EvalCount != 2 || fin.PromptEvalCount != 9 {
		t.Errorf("final chunk: %+v", fin)
	}
}

func TestChatUpstreamErrorPropagates(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Authentication Fails (no such user)","type":"authentication_error"}}`)
	})

	resp, err := http.Post(ts.URL+"/api/chat", "application/json",
		strings.NewReader(`{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
	var out struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Error, "Authentication Fails") {
		t.Errorf("error message not propagated: %q", out.Error)
	}
}

func TestChatValidation(t *testing.T) {
	ts := newTestServer(t, nil)
	resp, err := http.Post(ts.URL+"/api/chat", "application/json",
		strings.NewReader(`{"model":"deepseek-chat","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty messages: got status %d, want 400", resp.StatusCode)
	}
}
