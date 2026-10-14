package webengine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
)

// TestRunTurnExplainsExhaustedPlanNudges covers a real failure pattern seen
// live three times with gemma4:12b/Anvil (see docs/DECISIONS.md): the model
// writes real output but never calls plan_update, gets nudged up to
// maxPlanNudges times, and can still come back with empty content and no
// tool calls. Before this fix, finishRun(nil) ended the turn with nothing
// visible at all — the transcript just stopped. Now a system block explains
// what happened.
func TestRunTurnExplainsExhaustedPlanNudges(t *testing.T) {
	client := backend.NewClient("http://fake.test/v1")
	client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// Every turn: the model gives up silently (empty content, no tool calls).
		body := `{"message":{"role":"assistant","content":""},"done":true,"eval_count":1}` + "\n"
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	s := &Session{
		cfg:      config.Config{Backend: "ollama", BaseURL: "http://fake.test/v1", Model: "test-model"},
		client:   client,
		messages: []backend.Message{{Role: "user", Content: "do the thing"}},
		plan:     []planStep{{Text: "write the file", Status: stepInProgress}},
		status:   "generating",
	}
	s.runTurn(context.Background())

	if s.status != "idle" {
		t.Fatalf("status=%s, want idle", s.status)
	}
	var nudges, explains int
	for _, b := range s.blocks {
		if b.Kind != BlockSystem {
			continue
		}
		if strings.Contains(b.Text, "unfinished steps") {
			nudges++
		}
		if strings.Contains(b.Text, "stopped without a final answer") {
			explains++
		}
	}
	if nudges != maxPlanNudges {
		t.Fatalf("nudges=%d, want %d", nudges, maxPlanNudges)
	}
	if explains != 1 {
		t.Fatalf("expected exactly one explanatory block, got %d: %+v", explains, s.blocks)
	}
}

// TestRunTurnNudgesOnceOnEmptyReply covers qwen3:1.7b live behavior with no
// plan: after failed tool calls it replied with nothing and the turn ended
// silently. It now gets one reminder, and a visible note if that fails too.
func TestRunTurnNudgesOnceOnEmptyReply(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replies   []string
		wantCalls int
		wantStop  bool
	}{
		{"recovers", []string{"", "Done: wrote the files."}, 2, false},
		{"stays empty", []string{"", "", ""}, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := backend.NewClient("http://fake.test/v1")
			client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				content := tc.replies[calls]
				calls++
				body := `{"message":{"role":"assistant","content":"` + content + `"},"done":true,"eval_count":1}` + "\n"
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			s := &Session{
				cfg:      config.Config{Backend: "ollama", BaseURL: "http://fake.test/v1", Model: "test-model"},
				client:   client,
				messages: []backend.Message{{Role: "user", Content: "do the thing"}},
				status:   "generating",
			}
			s.runTurn(context.Background())
			if calls != tc.wantCalls {
				t.Fatalf("model calls=%d, want %d", calls, tc.wantCalls)
			}
			stopped := false
			for _, b := range s.blocks {
				if b.Kind == BlockSystem && strings.Contains(b.Text, "stopped without a final answer") {
					stopped = true
				}
			}
			if stopped != tc.wantStop {
				t.Fatalf("stopped note=%v, want %v: %+v", stopped, tc.wantStop, s.blocks)
			}
		})
	}
}
