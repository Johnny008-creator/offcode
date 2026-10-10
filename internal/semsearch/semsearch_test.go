package semsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func requireOllamaWithEmbedModel(t *testing.T) *backend.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test requires Ollama and nomic-embed-text")
	}
	client := backend.NewClient("http://localhost:11434/v1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.Embed(ctx, EmbedModel, []string{"probe"}); err != nil {
		t.Skipf("embedding model %s not available: %v", EmbedModel, err)
	}
	return client
}

func TestSearchFindsSemanticMatch(t *testing.T) {
	client := requireOllamaWithEmbedModel(t)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "auth.go"), `package auth

// ValidateSessionToken checks that a session token is well-formed and
// not expired, returning the associated user id if valid.
func ValidateSessionToken(token string) (userID string, err error) {
	if len(token) < 10 {
		return "", errExpired
	}
	return parseUserID(token), nil
}
`)
	mustWrite(t, filepath.Join(dir, "math.go"), `package mathutil

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	matches, reembedded, err := Search(ctx, client, dir, "function that checks if a user's session is valid", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if reembedded == 0 {
		t.Fatal("expected files to be embedded on first search")
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if matches[0].File != "auth.go" {
		t.Errorf("expected auth.go to rank first for a session-validation query, got %q (score %.3f)", matches[0].File, matches[0].Score)
	}
}

func TestSearchIncrementalReindex(t *testing.T) {
	client := requireOllamaWithEmbedModel(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() {}\n")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, n1, err := Search(ctx, client, dir, "anything", 5)
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if n1 == 0 {
		t.Fatal("expected the first search to embed at least one file")
	}

	_, n2, err := Search(ctx, client, dir, "anything", 5)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected zero re-embeds when nothing changed, got %d", n2)
	}

	// Touch the file with new content -> should trigger exactly one re-embed.
	time.Sleep(10 * time.Millisecond) // ensure a distinct mtime
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() { /* changed */ }\n")
	_, n3, err := Search(ctx, client, dir, "anything", 5)
	if err != nil {
		t.Fatalf("third search: %v", err)
	}
	if n3 != 1 {
		t.Errorf("expected exactly 1 re-embed after changing one file, got %d", n3)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	c := []float32{0, 1, 0}
	if sim := cosineSimilarity(a, b); sim < 0.999 {
		t.Errorf("identical vectors should have similarity ~1, got %v", sim)
	}
	if sim := cosineSimilarity(a, c); sim > 0.001 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %v", sim)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Deleting the last source file must also persist an empty index.
func TestSearchDropsDeletedFiles(t *testing.T) {
	client := backend.NewClient("http://embedding.test/v1")
	client.HTTPClient = &http.Client{Transport: embedTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"embeddings":[[1,0]]}`)), Header: make(http.Header)}, nil
	})}

	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	mustWrite(t, path, "package a")
	matches, _, err := Search(context.Background(), client, dir, "query", 8)
	if err != nil || len(matches) != 1 {
		t.Fatalf("initial search: %v, %v", matches, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	matches, n, err := Search(context.Background(), client, dir, "query", 8)
	if err != nil || len(matches) != 0 || n != 0 {
		t.Fatalf("after deletion: %v, %d, %v", matches, n, err)
	}
	if len(loadIndex(dir).Chunks) != 0 {
		t.Fatal("deleted chunks still cached")
	}
}

func TestWalkHiddenRootAndSkipSymlinks(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, ".project")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "a.go"), "package a")
	outside := filepath.Join(parent, "outside.go")
	mustWrite(t, outside, "package outside")
	if err := os.Symlink(outside, filepath.Join(dir, "link.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	files, err := walkTextFiles(dir)
	if err != nil || len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("files: %v, %v", files, err)
	}
}

type embedTransport func(*http.Request) (*http.Response, error)

func (f embedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeEmbedClient(t *testing.T, onRequest func([]string)) *backend.Client {
	t.Helper()
	client := backend.NewClient("http://embedding.test/v1")
	client.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if onRequest != nil {
			onRequest(payload.Input)
		}
		vectors := make([][]float32, len(payload.Input))
		for i := range vectors {
			vectors[i] = []float32{1, 0}
		}
		data, _ := json.Marshal(map[string]any{"embeddings": vectors})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
	})
	return client
}

func TestIncrementalSyncOffline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	mustWrite(t, path, "package a")
	calls := 0
	client := fakeEmbedClient(t, func(_ []string) { calls++ })
	for i, want := range []int{1, 0, 1} {
		if i == 2 {
			mustWrite(t, path, "package changed")
		}
		n, err := Sync(context.Background(), client, dir)
		if err != nil || n != want {
			t.Fatalf("sync %d: n=%d err=%v", i, n, err)
		}
	}
	if calls != 2 {
		t.Fatalf("expected 2 embedding requests, got %d", calls)
	}
}

func TestEditDuringEmbeddingIsReindexed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	mustWrite(t, path, "package a")
	changed := false
	client := fakeEmbedClient(t, func(_ []string) {
		if !changed {
			changed = true
			mustWrite(t, path, "package changed_during_embedding")
		}
	})
	for i := 0; i < 2; i++ {
		n, err := Sync(context.Background(), client, dir)
		if err != nil || n != 1 {
			t.Fatalf("sync %d: n=%d err=%v", i, n, err)
		}
	}
	if !strings.Contains(loadIndex(dir).Chunks[0].Text, "changed_during_embedding") {
		t.Fatal("stale content cached")
	}
}

func TestCanceledSyncLeavesCacheUntouched(t *testing.T) {
	dir := t.TempDir()
	client := fakeEmbedClient(t, nil)
	mustWrite(t, filepath.Join(dir, "a.go"), "package a")
	if _, err := Sync(context.Background(), client, dir); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(indexPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Sync(ctx, client, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	after, err := os.ReadFile(indexPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("cancellation modified cache")
	}
}

func TestInvalidProjectRoot(t *testing.T) {
	if _, err := walkTextFiles(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root was silently accepted")
	}
}
