package backend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type embedTransport func(*http.Request) (*http.Response, error)

func (f embedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEmbedProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"valid", `{"embeddings":[[1,0],[0,1]]}`, 200, false},
		{"missing model", `{"error":"model not found"}`, 404, true},
		{"model error", `{"error":"cannot embed"}`, 200, true},
		{"wrong count", `{"embeddings":[[1,0]]}`, 200, true},
		{"empty vector", `{"embeddings":[[],[]]}`, 200, true},
		{"dimensions", `{"embeddings":[[1,0],[1]]}`, 200, true},
		{"invalid json", `not json`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("http://embedding.test/v1/")
			client.HTTPClient.Transport = embedTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.URL.Path != "/api/embed" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body struct {
					Model string   `json:"model"`
					Input []string `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Model != "test-model" || len(body.Input) != 2 {
					t.Fatalf("wrong payload: %+v", body)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			vectors, err := client.Embed(context.Background(), "test-model", []string{"a", "b"})
			if (err != nil) != tc.wantError {
				t.Fatalf("vectors=%v error=%v", vectors, err)
			}
		})
	}
}
