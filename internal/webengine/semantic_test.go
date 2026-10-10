package webengine

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
)

type semanticTransport func(*http.Request) (*http.Response, error)

func (f semanticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSemanticSearchDispatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte("package auth\nfunc ValidateSession() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client := backend.NewClient("http://embedding.test/v1")
	client.HTTPClient.Transport = semanticTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/embed" {
			t.Fatalf("unexpected endpoint %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"embeddings":[[1,0]]}`)), Header: make(http.Header)}, nil
	})
	s := &Session{cfg: config.Config{Backend: "ollama"}, client: client, cwd: dir}
	s.runToolCall(backend.ToolCall{ID: "search-1", Function: backend.FunctionCall{Name: "semantic_search", Arguments: `{"query":"session validation"}`}})
	if len(s.messages) != 1 {
		t.Fatalf("expected one tool result, got %d", len(s.messages))
	}
	result := s.messages[0]
	if result.ToolCallID != "search-1" || !strings.Contains(result.Content, "auth.go:1-") || !strings.Contains(result.Content, "ValidateSession") {
		t.Fatalf("unexpected result: %+v", result)
	}
}
