package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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
	Images    []string         `json:"images,omitempty"`
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
	Options  *nativeOptions  `json:"options,omitempty"`
}

type nativeOptions struct {
	NumCtx int `json:"num_ctx,omitempty"`
}

// defaultNumCtx overrides Ollama's own VRAM-based auto-sizing, which can be
// far too small for a tool-calling agent — observed live on an 8GB card:
// Ollama picked 4096 tokens by default, and offcode's own system prompt
// (tool schemas alone) is already ~2K tokens, so a reasoning-heavy turn
// (gemma4:12b/Anvil) silently hit "truncated = 1" mid-response with no
// error surfaced — the turn just ended with a dangling thinking trace and
// no answer. 8192 is a deliberately modest bump (not the model's full
// advertised context_length, which can be 256K+) to stay safe on small-VRAM
// hardware; override with OFFCODE_NUM_CTX for a different tradeoff.
const defaultNumCtx = 8192

func numCtxOption() *nativeOptions {
	n := defaultNumCtx
	if v := os.Getenv("OFFCODE_NUM_CTX"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			n = parsed
		}
	}
	if n == 0 {
		return nil
	}
	return &nativeOptions{NumCtx: n}
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
	EvalDuration    int64  `json:"eval_duration"`
	Error           string `json:"error"`
}

func toNativeMessages(msgs []Message) []nativeMessage {
	out := make([]nativeMessage, len(msgs))
	for i, m := range msgs {
		nm := nativeMessage{Role: m.Role, Content: m.Content, Images: m.Images}
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
			Options:  numCtxOption(),
		}
		if _, explicit := os.LookupEnv("OFFCODE_NUM_CTX"); !explicit && nreq.Options != nil && c.modelfileSetsNumCtx(ctx, req.Model) {
			// The model's own Modelfile picked a context; don't override it.
			nreq.Options = nil
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
					EvalDurationNs:   chunk.EvalDuration,
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

// modelfileSetsNumCtx reports whether the model's Modelfile parameters set
// num_ctx, via Ollama's /api/show. Cached per model; lookup failures count
// as "not set" so offcode's default still protects against truncation.
func (c *Client) modelfileSetsNumCtx(ctx context.Context, model string) bool {
	c.modelfileCtxMu.Lock()
	if v, ok := c.modelfileCtx[model]; ok {
		c.modelfileCtxMu.Unlock()
		return v
	}
	c.modelfileCtxMu.Unlock()

	set := false
	body, _ := json.Marshal(map[string]string{"model": model})
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	if httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/show", bytes.NewReader(body)); err == nil {
		httpReq.Header.Set("Content-Type", "application/json")
		if resp, err := c.HTTPClient.Do(httpReq); err == nil {
			var parsed struct {
				Parameters string `json:"parameters"`
			}
			if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&parsed) == nil {
				for _, line := range strings.Split(parsed.Parameters, "\n") {
					if f := strings.Fields(line); len(f) > 0 && f[0] == "num_ctx" {
						set = true
					}
				}
			}
			resp.Body.Close()
		}
	}

	c.modelfileCtxMu.Lock()
	if c.modelfileCtx == nil {
		c.modelfileCtx = map[string]bool{}
	}
	c.modelfileCtx[model] = set
	c.modelfileCtxMu.Unlock()
	return set
}
