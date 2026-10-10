package backend

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAICompatibleStreamingToolArguments(t *testing.T) {
	c := NewClient("http://lmstudio.test/v1")
	c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("endpoint=%s", r.URL.Path)
		}
		// A tool name/JSON argument split across chunks must be reassembled.
		body := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call1","type":"function","function":{"name":"read","arguments":"{\"pa"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"a.txt\"}"}}]}}]}

data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}

data: [DONE]

`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	done := false
	for ev := range c.StreamChatCompletion(context.Background(), ChatRequest{Model: "local"}) {
		if ev.Kind == EventError {
			t.Fatal(ev.Err)
		}
		if ev.Kind == EventDone {
			done = true
			if len(ev.Message.ToolCalls) != 1 {
				t.Fatalf("calls=%v", ev.Message.ToolCalls)
			}
			call := ev.Message.ToolCalls[0]
			if call.Function.Name != "read" || call.Function.Arguments != `{"path":"a.txt"}` || call.ID != "call1" || ev.Usage.TotalTokens != 13 {
				t.Fatalf("event=%+v", ev)
			}
		}
	}
	if !done {
		t.Fatal("no final event")
	}
}
