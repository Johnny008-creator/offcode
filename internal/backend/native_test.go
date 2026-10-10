package backend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

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
