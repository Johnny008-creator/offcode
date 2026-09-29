package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type StreamEventKind int

const (
	EventContent StreamEventKind = iota
	EventReasoning
	EventToolCallStart
	EventDone
	EventError
)

type StreamEvent struct {
	Kind         StreamEventKind
	ContentDelta string
	ToolCallName string
	Message      Message
	Usage        Usage
	Err          error
}

// StreamChatCompletion streams a chat completion, emitting one StreamEvent
// per SSE chunk on the returned channel, and a final EventDone (or
// EventError) before the channel is closed. The channel is always closed,
// even on context cancellation, so callers can range over it safely.
func (c *Client) StreamChatCompletion(ctx context.Context, req ChatRequest) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		req.Stream = true
		req.StreamOptions = &streamOptions{IncludeUsage: true}

		body, err := json.Marshal(req)
		if err != nil {
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("marshal request: %w", err)}
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
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
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("request to %s: %w", c.BaseURL, err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("backend returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))}
			return
		}

		var contentBuf strings.Builder
		toolCalls := map[int]*ToolCall{}
		var toolOrder []int
		var usage Usage
		announced := map[int]bool{}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				break
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if chunk.Error != nil {
				events <- StreamEvent{Kind: EventError, Err: fmt.Errorf("backend error: %s", chunk.Error.Message)}
				return
			}
			if chunk.Usage != nil {
				usage = *chunk.Usage
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				contentBuf.WriteString(delta.Content)
				events <- StreamEvent{Kind: EventContent, ContentDelta: delta.Content}
			}
			if delta.Reasoning != "" {
				events <- StreamEvent{Kind: EventReasoning, ContentDelta: delta.Reasoning}
			}
			for _, tc := range delta.ToolCalls {
				existing, ok := toolCalls[tc.Index]
				if !ok {
					copyTC := tc
					toolCalls[tc.Index] = &copyTC
					toolOrder = append(toolOrder, tc.Index)
					existing = toolCalls[tc.Index]
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					existing.Function.Arguments += tc.Function.Arguments
				}
				if existing.Function.Name != "" && !announced[tc.Index] {
					announced[tc.Index] = true
					events <- StreamEvent{Kind: EventToolCallStart, ToolCallName: existing.Function.Name}
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

		sort.Ints(toolOrder)
		var finalToolCalls []ToolCall
		for _, idx := range toolOrder {
			finalToolCalls = append(finalToolCalls, *toolCalls[idx])
		}

		events <- StreamEvent{
			Kind: EventDone,
			Message: Message{
				Role:      "assistant",
				Content:   contentBuf.String(),
				ToolCalls: finalToolCalls,
			},
			Usage: usage,
		}
	}()

	return events
}
