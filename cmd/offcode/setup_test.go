package main

import (
	"context"
	"github.com/Johnny008-creator/offcode/internal/setup"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSelectedModels(t *testing.T) {
	got, err := selectedModels("spark,forge,spark,embedding,custom:7b")
	want := []string{"qwen3:1.7b", "qwen3:8b", "nomic-embed-text", "custom:7b"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	got, err = selectedModels("all")
	if err != nil || len(got) != 5 {
		t.Fatal(got, err)
	}
	for _, input := range []string{"forge,", "-bad", "forge,none", "$(oops)"} {
		if _, err = selectedModels(input); err == nil {
			t.Fatal("accepted", input)
		}
	}
}
func TestPullModel(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
		ok         bool
	}{
		{"success", "{\"status\":\"pulling\",\"total\":100,\"completed\":10}\n{\"status\":\"success\"}\n", 200, true},
		{"backend error", `{"error":"not found"}`, 200, false},
		{"truncated", `{"status":"pulling"}`, 200, false},
		{"http failure", "bad", 500, false},
		{"bad json", "no", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/pull" || r.Method != "POST" {
					t.Error(r.URL, r.Method)
				}
				w.WriteHeader(tc.code)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			err := pullModel(context.Background(), srv.URL, "test:1b", io.Discard)
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
		})
	}
}

func TestSetupPreservesOnlySelectedGroups(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OLLAMA_HOST", "127.0.0.1:12345")
	t.Setenv("OLLAMA_VULKAN", "1")
	if err := configureInstallation([]string{"-preserve=server", "-models=none", "-updates=off"}); err != nil {
		t.Fatal(err)
	}
	saved, err := setup.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Selection() != "server" || len(saved.OllamaEnv) != 1 || saved.OllamaEnv["OLLAMA_HOST"] != "127.0.0.1:12345" {
		t.Fatal(saved)
	}
	// An unrelated setup change must retain the checkbox selection.
	if err = configureInstallation([]string{"-ollama=preserve", "-updates=notify"}); err != nil {
		t.Fatal(err)
	}
	saved, err = setup.Load()
	if err != nil || saved.Selection() != "server" {
		t.Fatal(saved, err)
	}
	if err = configureInstallation([]string{"-preserve=none"}); err != nil {
		t.Fatal(err)
	}
	saved, err = setup.Load()
	if err != nil || saved.Preserve == nil || len(saved.Preserve) != 0 || len(saved.OllamaEnv) != 0 {
		t.Fatal(saved, err)
	}
}
