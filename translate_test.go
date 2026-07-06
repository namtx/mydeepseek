package main

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
}

func newTestTranslator(model string) *streamTranslator {
	tr := newStreamTranslator(model)
	tr.now = fixedNow
	return tr
}

func TestTranslateContentChunk(t *testing.T) {
	tr := newTestTranslator("deepseek-chat")
	out := tr.translate([]byte(`data: {"model":"deepseek-chat","choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`))
	if len(out) != 1 {
		t.Fatalf("expected 1 response, got %d", len(out))
	}
	if out[0].Message.Content != "Hello" || out[0].Message.Thinking != "" || out[0].Done {
		t.Errorf("unexpected response: %+v", out[0])
	}
}

func TestTranslateReasoningChunk(t *testing.T) {
	tr := newTestTranslator("deepseek-reasoner")
	out := tr.translate([]byte(`data: {"choices":[{"delta":{"reasoning_content":"Let me think."},"finish_reason":null}]}`))
	if len(out) != 1 {
		t.Fatalf("expected 1 response, got %d", len(out))
	}
	if out[0].Message.Thinking != "Let me think." || out[0].Message.Content != "" {
		t.Errorf("reasoning_content not mapped to thinking: %+v", out[0])
	}
	if out[0].Model != "deepseek-reasoner" {
		t.Errorf("model fallback not applied: %q", out[0].Model)
	}
}

func TestTranslateFinishWithUsageSameChunk(t *testing.T) {
	tr := newTestTranslator("deepseek-chat")
	out := tr.translate([]byte(`data: {"choices":[{"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	if len(out) != 2 {
		t.Fatalf("expected content + final, got %d responses", len(out))
	}
	fin := out[1]
	if !fin.Done || fin.DoneReason != "stop" || fin.EvalCount != 5 || fin.PromptEvalCount != 10 {
		t.Errorf("unexpected final response: %+v", fin)
	}
	// [DONE] afterwards must not produce a second final chunk.
	if extra := tr.translate([]byte("data: [DONE]")); len(extra) != 0 {
		t.Errorf("final response emitted twice: %+v", extra)
	}
}

func TestTranslateUsageInTrailingChunk(t *testing.T) {
	tr := newTestTranslator("deepseek-chat")
	if out := tr.translate([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)); len(out) != 0 {
		t.Fatalf("final should be held back until usage arrives, got %+v", out)
	}
	out := tr.translate([]byte(`data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3}}`))
	if len(out) != 1 || !out[0].Done || out[0].EvalCount != 3 || out[0].PromptEvalCount != 7 {
		t.Fatalf("unexpected final from usage chunk: %+v", out)
	}
}

func TestTranslateDoneWithoutUsage(t *testing.T) {
	tr := newTestTranslator("deepseek-chat")
	tr.translate([]byte(`data: {"choices":[{"delta":{},"finish_reason":"length"}]}`))
	out := tr.translate([]byte("data: [DONE]"))
	if len(out) != 1 || !out[0].Done || out[0].DoneReason != "length" {
		t.Fatalf("expected final with done_reason=length, got %+v", out)
	}
}

func TestTranslateIgnoresNoise(t *testing.T) {
	tr := newTestTranslator("deepseek-chat")
	for _, line := range []string{
		"",
		": keep-alive",
		"event: message",
		"data: {not json",
		`data: {"choices":[{"delta":{},"finish_reason":null}]}`,
	} {
		if out := tr.translate([]byte(line)); len(out) != 0 {
			t.Errorf("line %q should yield nothing, got %+v", line, out)
		}
	}
}

func TestToDeepSeekRequest(t *testing.T) {
	temp := 0.7
	numPredict := 256
	req := ollamaChatRequest{
		Model: "deepseek-chat",
		Messages: []ollamaMessage{
			{Role: "user", Content: "hi", Thinking: "should be dropped"},
		},
		Options: &ollamaOptions{Temperature: &temp, NumPredict: &numPredict, Stop: []string{"END"}},
	}
	ds := toDeepSeekRequest(req, true)
	if ds.Model != "deepseek-chat" || !ds.Stream {
		t.Errorf("model/stream not mapped: %+v", ds)
	}
	if ds.StreamOptions == nil || !ds.StreamOptions.IncludeUsage {
		t.Error("stream_options.include_usage should be set when streaming")
	}
	if *ds.Temperature != 0.7 || *ds.MaxTokens != 256 || len(ds.Stop) != 1 {
		t.Errorf("options not mapped: %+v", ds)
	}
	if len(ds.Messages) != 1 || ds.Messages[0].Content != "hi" {
		t.Errorf("messages not mapped: %+v", ds.Messages)
	}
	nonStream := toDeepSeekRequest(req, false)
	if nonStream.Stream || nonStream.StreamOptions != nil {
		t.Error("non-streaming request must have stream=false and no stream_options")
	}
}
