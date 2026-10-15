package backend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStreamChatNativeRejectsIncompleteStream(t *testing.T) {
	for _, body := range []string{
		"",
		`{"message":{"content":"partial reply"},"done":false}` + "\n",
		`{"message":{"tool_calls":[{"function":{"name":"write","arguments":{"path":"x","content":"partial"}}}]},"done":false}` + "\n",
	} {
		t.Run(body, func(t *testing.T) {
			c := NewClient("http://ollama.test/v1")
			c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			gotError := false
			for ev := range c.StreamChatNative(context.Background(), ChatRequest{Model: "test"}, ThinkDefault) {
				if ev.Kind == EventDone {
					t.Fatal("incomplete stream must not finalize a reply or tool calls")
				}
				if ev.Kind == EventError {
					gotError = errors.Is(ev.Err, io.ErrUnexpectedEOF)
				}
			}
			if !gotError {
				t.Fatal("expected unexpected EOF error")
			}
		})
	}
}

// TestStreamChatNativeSetsNumCtx covers a real failure found live
// (2026-10-10, see docs/DECISIONS.md): without an explicit num_ctx,
// Ollama's own VRAM-based auto-sizing picked a 4096-token window on an
// 8GB card, and a reasoning-heavy turn (gemma4:12b) got silently
// truncated mid-response with no error — just a dangling thinking trace
// and no final answer.
func TestStreamChatNativeSetsNumCtx(t *testing.T) {
	c := NewClient("http://ollama.test/v1")
	var gotOptions json.RawMessage
	c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Options json.RawMessage `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotOptions = body.Options
		resp := `{"message":{"role":"assistant","content":"ok"},"done":true}` + "\n"
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
	})
	for ev := range c.StreamChatNative(context.Background(), ChatRequest{Model: "test"}, ThinkDefault) {
		if ev.Kind == EventError {
			t.Fatal(ev.Err)
		}
	}
	var opts struct {
		NumCtx int `json:"num_ctx"`
	}
	if err := json.Unmarshal(gotOptions, &opts); err != nil {
		t.Fatalf("no options sent: %v", err)
	}
	if opts.NumCtx != defaultNumCtx {
		t.Fatalf("num_ctx=%d, want default %d", opts.NumCtx, defaultNumCtx)
	}
}

func TestStreamChatNativeNumCtxEnvOverride(t *testing.T) {
	t.Setenv("OFFCODE_NUM_CTX", "32768")
	c := NewClient("http://ollama.test/v1")
	var gotOptions json.RawMessage
	c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Options json.RawMessage `json:"options"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotOptions = body.Options
		resp := `{"message":{"role":"assistant","content":"ok"},"done":true}` + "\n"
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
	})
	for ev := range c.StreamChatNative(context.Background(), ChatRequest{Model: "test"}, ThinkDefault) {
		if ev.Kind == EventError {
			t.Fatal(ev.Err)
		}
	}
	var opts struct {
		NumCtx int `json:"num_ctx"`
	}
	if err := json.Unmarshal(gotOptions, &opts); err != nil {
		t.Fatalf("no options sent: %v", err)
	}
	if opts.NumCtx != 32768 {
		t.Fatalf("num_ctx=%d, want 32768 from OFFCODE_NUM_CTX override", opts.NumCtx)
	}
}

func TestPreservedOllamaOptions(t *testing.T) {
	t.Setenv("OFFCODE_NUM_CTX", "0")
	req := nativeChatRequest{Model: "test", Options: numCtxOption()}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "num_ctx") || strings.Contains(string(data), "num_gpu") || strings.Contains(string(data), "options") {
		t.Fatalf("must leave server settings untouched: %s", data)
	}
}

// A Modelfile's own num_ctx is the user's choice and must not be overridden
// by offcode's default, unless OFFCODE_NUM_CTX is set explicitly.
func TestStreamChatNativeRespectsModelfileNumCtx(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		want      bool // options.num_ctx sent
	}{
		{"modelfile wins over default", "", false},
		{"explicit env wins over modelfile", "16384", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("OFFCODE_NUM_CTX", tc.env)
			}
			c := NewClient("http://ollama.test/v1")
			var chatBody string
			c.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/show" {
					resp := `{"parameters":"num_ctx                        32768\ntemperature                    1"}`
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
				}
				raw, _ := io.ReadAll(r.Body)
				chatBody = string(raw)
				resp := `{"message":{"role":"assistant","content":"ok"},"done":true}` + "\n"
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resp)), Header: make(http.Header)}, nil
			})
			for ev := range c.StreamChatNative(context.Background(), ChatRequest{Model: "custom"}, ThinkDefault) {
				if ev.Kind == EventError {
					t.Fatal(ev.Err)
				}
			}
			if got := strings.Contains(chatBody, "num_ctx"); got != tc.want {
				t.Fatalf("num_ctx sent=%v, want %v: %s", got, tc.want, chatBody)
			}
		})
	}
}
