package main

import (
	"bytes"
	"encoding/json"
	"time"
)

var (
	ssePrefix  = []byte("data: ")
	sseDone    = []byte("[DONE]")
	timeFormat = time.RFC3339Nano
)

// streamTranslator converts DeepSeek SSE chunks into Ollama NDJSON chat
// responses. DeepSeek may deliver usage in the same chunk as finish_reason
// or in a trailing usage-only chunk, so the final Ollama response (done:true)
// is held back until usage arrives or the stream ends.
type streamTranslator struct {
	model      string // fallback when a chunk omits the model
	doneReason string
	finished   bool // saw a finish_reason; final response pending
	emittedFin bool
	now        func() time.Time
}

func newStreamTranslator(model string) *streamTranslator {
	return &streamTranslator{model: model, now: time.Now}
}

// translate parses one SSE line and returns zero or more Ollama responses.
// Blank lines, comments, malformed JSON, and empty deltas yield nothing.
func (t *streamTranslator) translate(line []byte) []ollamaChatResponse {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, ssePrefix) {
		return nil
	}
	payload := bytes.TrimSpace(line[len(ssePrefix):])
	if bytes.Equal(payload, sseDone) {
		return t.finish(nil)
	}

	var chunk dsStreamChunk
	if err := json.Unmarshal(payload, &chunk); err != nil {
		return nil
	}
	if chunk.Model != "" {
		t.model = chunk.Model
	}

	// Usage-only chunk (empty choices) closes a pending final response.
	if len(chunk.Choices) == 0 {
		if t.finished && chunk.Usage != nil {
			return t.finish(chunk.Usage)
		}
		return nil
	}

	choice := chunk.Choices[0]
	var out []ollamaChatResponse
	if choice.Delta.Content != "" || choice.Delta.ReasoningContent != "" {
		out = append(out, ollamaChatResponse{
			Model:     t.model,
			CreatedAt: t.now().UTC().Format(timeFormat),
			Message: ollamaMessage{
				Role:     "assistant",
				Content:  choice.Delta.Content,
				Thinking: choice.Delta.ReasoningContent,
			},
		})
	}
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		t.finished = true
		t.doneReason = *choice.FinishReason
		if chunk.Usage != nil {
			out = append(out, t.finish(chunk.Usage)...)
		}
	}
	return out
}

// finish emits the single done:true response, at most once per stream.
func (t *streamTranslator) finish(usage *dsUsage) []ollamaChatResponse {
	if t.emittedFin {
		return nil
	}
	t.emittedFin = true
	if t.doneReason == "" {
		t.doneReason = "stop"
	}
	resp := ollamaChatResponse{
		Model:      t.model,
		CreatedAt:  t.now().UTC().Format(timeFormat),
		Message:    ollamaMessage{Role: "assistant"},
		Done:       true,
		DoneReason: t.doneReason,
	}
	if usage != nil {
		resp.EvalCount = usage.CompletionTokens
		resp.PromptEvalCount = usage.PromptTokens
	}
	return []ollamaChatResponse{resp}
}

// toDeepSeekRequest maps an Ollama chat request onto DeepSeek's API.
// Thinking text in history is dropped: DeepSeek expects role+content only.
func toDeepSeekRequest(req ollamaChatRequest, stream bool) dsChatRequest {
	out := dsChatRequest{
		Model:  req.Model,
		Stream: stream,
	}
	if stream {
		out.StreamOptions = &dsStreamOptions{IncludeUsage: true}
	}
	for _, m := range req.Messages {
		out.Messages = append(out.Messages, dsMessage{Role: m.Role, Content: m.Content})
	}
	if o := req.Options; o != nil {
		out.Temperature = o.Temperature
		out.TopP = o.TopP
		out.MaxTokens = o.NumPredict
		out.Stop = o.Stop
	}
	return out
}
