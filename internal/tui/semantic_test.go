package tui

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type semanticTransport func(*http.Request) (*http.Response, error)

func (f semanticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSemanticSearchCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte("package auth\nfunc ValidateSession() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client := backend.NewClient("http://embedding.test/v1")
	client.HTTPClient.Transport = semanticTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"embeddings":[[1,0]]}`)), Header: make(http.Header)}, nil
	})
	tc := backend.ToolCall{ID: "search-1", Function: backend.FunctionCall{Name: "semantic_search"}}
	result := execSemanticSearchCmd(context.Background(), tc, client, dir, "session validation", 8)().(semanticSearchDoneMsg)
	if result.err != nil || result.callID != tc.ID || result.reembedded != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	text := renderSemanticMatches(result.matches, result.reembedded)
	if !strings.Contains(text, "auth.go:1-") || !strings.Contains(text, "ValidateSession") {
		t.Fatalf("unexpected rendered output: %s", text)
	}
}
