package main

// Ollama wire types (what the client sends and expects back).

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	// Stream defaults to true when absent, matching real Ollama.
	Stream  *bool          `json:"stream"`
	Options *ollamaOptions `json:"options"`
}

type ollamaMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

// ollamaOptions carries only the sampling parameters DeepSeek accepts;
// everything else the client sends is ignored (see CONTEXT.md: Translation).
type ollamaOptions struct {
	Temperature *float64 `json:"temperature"`
	TopP        *float64 `json:"top_p"`
	NumPredict  *int     `json:"num_predict"`
	Stop        []string `json:"stop"`
}

type ollamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
}

// DeepSeek wire types (OpenAI-compatible chat completions).

type dsChatRequest struct {
	Model         string           `json:"model"`
	Messages      []dsMessage      `json:"messages"`
	Stream        bool             `json:"stream"`
	StreamOptions *dsStreamOptions `json:"stream_options,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	MaxTokens     *int             `json:"max_tokens,omitempty"`
	Stop          []string         `json:"stop,omitempty"`
}

type dsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type dsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type dsChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *dsUsage `json:"usage"`
}

type dsStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *dsUsage `json:"usage"`
}

type dsErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
