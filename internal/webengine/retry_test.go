package webengine

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestSession(transport roundTripFunc) *Session {
	client := backend.NewClient("http://fake.test/v1")
	client.HTTPClient.Transport = transport
	return &Session{
		cfg:      config.Config{Backend: "ollama", BaseURL: "http://fake.test/v1", Model: "test-model"},
		client:   client,
		messages: []backend.Message{{Role: "user", Content: "hi"}},
	}
}

// TestStreamOnceWithParseRetryRecoversFromMalformedToolCall simulates Ollama's
// native /api/chat failing to parse a model's own tool-call tags (observed
// live with qwen3.5:4b, see docs/DECISIONS.md) on the first attempt and
// succeeding on a retry — the turn should recover instead of aborting.
func TestStreamOnceWithParseRetryRecoversFromMalformedToolCall(t *testing.T) {
	var calls atomic.Int32
	s := newTestSession(func(r *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		var body string
		if n == 1 {
			body = `{"error":"XML syntax error on line 5: element <function> closed by </parameter>"}` + "\n"
		} else {
			body = `{"message":{"role":"assistant","content":"all good"},"done":true,"eval_count":3}` + "\n"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	msg, _, _, err := s.streamOnceWithParseRetry(t.Context())
	if err != nil {
		t.Fatalf("expected recovery, got error: %v", err)
	}
	if msg.Content != "all good" {
		t.Fatalf("content=%q", msg.Content)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want exactly one retry", calls.Load())
	}
}

// TestStreamOnceWithParseRetryGivesUpAfterLimit confirms the retry is
// bounded: a model/template that fails every time must not retry forever.
func TestStreamOnceWithParseRetryGivesUpAfterLimit(t *testing.T) {
	var calls atomic.Int32
	s := newTestSession(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := `{"error":"XML syntax error on line 1: unexpected EOF"}` + "\n"
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	_, _, _, err := s.streamOnceWithParseRetry(t.Context())
	if err == nil || !backend.IsMalformedToolCallResponse(err) {
		t.Fatalf("expected a surfaced malformed-tool-call error, got %v", err)
	}
	if want := int32(maxToolCallParseRetries + 1); calls.Load() != want {
		t.Fatalf("calls=%d, want %d (initial attempt + %d retries)", calls.Load(), want, maxToolCallParseRetries)
	}
}

// TestStreamOnceWithParseRetryPassesThroughOtherErrors confirms unrelated
// failures (backend unreachable, model missing) are not retried and are
// not mistaken for the tool-call-parsing case.
func TestStreamOnceWithParseRetryPassesThroughOtherErrors(t *testing.T) {
	var calls atomic.Int32
	s := newTestSession(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("internal error")), Header: make(http.Header)}, nil
	})

	_, _, _, err := s.streamOnceWithParseRetry(t.Context())
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d, want exactly 1 (no retry for an unrelated error)", calls.Load())
	}
}
