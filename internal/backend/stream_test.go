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

func TestStreamingReasoningContentField(t *testing.T) {
	c := NewClient("http://lmstudio.test/v1")
	c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		// LM Studio streams the thinking trace as reasoning_content.
		body := `data: {"choices":[{"delta":{"role":"assistant","reasoning_content":"Okay"}}]}

data: {"choices":[{"delta":{"reasoning":", so"}}]}

data: {"choices":[{"delta":{"content":"4"}}]}

data: [DONE]

`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	var reasoning, content string
	for ev := range c.StreamChatCompletion(context.Background(), ChatRequest{Model: "local"}) {
		switch ev.Kind {
		case EventError:
			t.Fatal(ev.Err)
		case EventReasoning:
			reasoning += ev.ContentDelta
		case EventDone:
			content = ev.Message.Content
		}
	}
	if reasoning != "Okay, so" || content != "4" {
		t.Fatalf("reasoning=%q content=%q", reasoning, content)
	}
}

func TestLMStudioModelMetadata(t *testing.T) {
	c := NewClient("http://lmstudio.test/v1")
	c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v0/models" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("Unexpected endpoint")), Header: make(http.Header)}, nil
		}
		body := `{"data":[{"id":"qwen3-1.7b","type":"llm","state":"loaded","max_context_length":40960,"loaded_context_length":16384},
{"id":"gemma","type":"vlm","state":"not-loaded","max_context_length":8192},
{"id":"text-embedding-nomic","type":"embeddings","max_context_length":2048}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	names, err := c.ListModels(context.Background(), "lmstudio")
	if err != nil || strings.Join(names, ",") != "qwen3-1.7b,gemma" {
		t.Fatalf("names=%v err=%v", names, err)
	}
	if n := c.ContextLength(context.Background(), "qwen3-1.7b"); n != 16384 {
		t.Fatalf("loaded context=%d", n)
	}
	if n := c.ContextLength(context.Background(), "gemma"); n != 8192 {
		t.Fatalf("unloaded context=%d", n)
	}
}
