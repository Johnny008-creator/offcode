package backend

import (
	"encoding/json"
	"strings"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`

	// Images are base64-encoded (no data: URI prefix) image attachments,
	// only honored on the native Ollama path (StreamChatNative) — see
	// toNativeMessages. The OpenAI-compatible path would need a
	// structured content array instead of a plain string to support
	// this, which isn't implemented since that backend is unverified
	// for anything in this project so far.
	Images []string `json:"-"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Index    int          `json:"index,omitempty"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ChatRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Tools         []Tool         `json:"tools,omitempty"`
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// EvalDurationNs is pure generation time in nanoseconds, as reported by
	// Ollama's native API (eval_duration) — excludes prompt processing.
	// Zero when the backend doesn't report it (e.g. the OpenAI-compatible
	// /v1 path), in which case callers fall back to wall-clock timing.
	EvalDurationNs int64
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// IsMalformedToolCallResponse reports whether err is Ollama's native /api/chat
// failing to parse a model's own tool-call output against its chat
// template — observed live as "XML syntax error on line N: ..." from a
// model (e.g. qwen3.5:4b) whose generated <function>/<parameter> tags came
// out unbalanced. The model produced a real response; only Ollama's
// server-side extraction of it failed, so a fresh attempt (different
// sampling) has a real chance of succeeding — unlike a backend-down or
// model-not-found error, which a retry can't fix and shouldn't swallow.
func IsMalformedToolCallResponse(err error) bool {
	return err != nil && strings.Contains(err.Error(), "XML syntax error")
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string     `json:"role,omitempty"`
			Content   string     `json:"content,omitempty"`
			Reasoning string     `json:"reasoning,omitempty"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
