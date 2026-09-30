package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ThinkLevel controls Ollama's native reasoning-effort knob for hybrid
// reasoning models like qwen3. Only honored on the native /api/chat path
// (StreamChatNative) — Ollama's OpenAI-compatible /v1 endpoint silently
// ignores a "think" field, confirmed by live testing.
type ThinkLevel string

const (
	ThinkDefault ThinkLevel = ""
	ThinkOff     ThinkLevel = "off"
	ThinkLow     ThinkLevel = "low"
	ThinkMedium  ThinkLevel = "medium"
	ThinkHigh    ThinkLevel = "high"
)

func (t ThinkLevel) nativeValue() any {
	switch t {
	case ThinkDefault:
		return nil
	case ThinkOff:
		return false
	default:
		return string(t)
	}
}

type nativeMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []nativeToolCall `json:"tool_calls,omitempty"`
}

type nativeToolCall struct {
	ID       string             `json:"id,omitempty"`
	Function nativeFunctionCall `json:"function"`
}

type nativeFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type nativeChatRequest struct {
	Model    string          `json:"model"`
	Messages []nativeMessage `json:"messages"`
	Tools    []Tool          `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Think    any             `json:"think,omitempty"`
}

type nativeStreamChunk struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		Thinking  string           `json:"thinking"`
		ToolCalls []nativeToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done            bool   `json:"done"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	Error           string `json:"error"`
}

func toNativeMessages(msgs []Message) []nativeMessage {
	out := make([]nativeMessage, len(msgs))
	for i, m := range msgs {
		nm := nativeMessage{Role: m.Role, Content: m.Content}
		for _, tc := range m.ToolCalls {
			args := tc.Function.Arguments
			if args == "" {
				args = "{}"
			}
			nm.ToolCalls = append(nm.ToolCalls, nativeToolCall{
				Function: nativeFunctionCall{
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(args),
				},
			})
		}
		out[i] = nm
	}
	return out
}

// StreamChatNative streams a chat completion via Ollama's native /api/chat
// endpoint (NDJSON, one complete JSON object per line — not SSE), the only
// path that honors the "think" field for controlling reasoning effort.
func (c *Client) StreamChatNative(ctx context.Context, req ChatRequest, think ThinkLevel) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		nreq := nativeChatRequest{
			Model:    req.Model,
			Messages: toNativeMessages(req.Messages),
			Tools:    req.Tools,
			Stream:   true,
			Think:    think.nativeValue(),
		}

		body, err := json.Marshal(nreq)
		if err != nil {
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("marshal request: %w", err)}
			return
		}

		root := strings.TrimSuffix(c.BaseURL, "/v1")
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/chat", bytes.NewReader(body))
		if err != nil {
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("build request: %w", err)}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			if ctx.Err() != nil {
				events <- StreamEvent{Kind: EventError, Err: ctx.Err()}
				return
			}
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("request to %s: %w", root, err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("backend returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))}
			return
		}

		var contentBuf strings.Builder
		var toolCalls []ToolCall
		var usage Usage

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var chunk nativeStreamChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}
			if chunk.Error != "" {
				events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("backend error: %s", chunk.Error)}
				return
			}

			if chunk.Message.Content != "" {
				contentBuf.WriteString(chunk.Message.Content)
				events <- StreamEvent{Kind: EventContent, ContentDelta: chunk.Message.Content}
			}
			if chunk.Message.Thinking != "" {
				events <- StreamEvent{Kind: EventReasoning, ContentDelta: chunk.Message.Thinking}
			}
			for _, tc := range chunk.Message.ToolCalls {
				args := "{}"
				if len(tc.Function.Arguments) > 0 {
					args = string(tc.Function.Arguments)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:       tc.ID,
					Type:     "function",
					Function: FunctionCall{Name: tc.Function.Name, Arguments: args},
				})
				events <- StreamEvent{Kind: EventToolCallStart, ToolCallName: tc.Function.Name}
			}
			if chunk.Done {
				usage = Usage{
					PromptTokens:     chunk.PromptEvalCount,
					CompletionTokens: chunk.EvalCount,
					TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
				}
			}
		}

		if err := scanner.Err(); err != nil {
			if ctx.Err() != nil {
				events <- StreamEvent{Kind: EventError, Err: ctx.Err()}
				return
			}
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("read stream: %w", err)}
			return
		}

		events <- StreamEvent{
			Kind: EventDone,
			Message: Message{
				Role:      "assistant",
				Content:   contentBuf.String(),
				ToolCalls: toolCalls,
			},
			Usage: usage,
		}
	}()

	return events
}
