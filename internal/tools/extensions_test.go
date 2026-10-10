package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProduceFileDoesNotOverwriteOrPublishFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.docx")
	fail := func(tmp string) error {
		if err := os.WriteFile(tmp, []byte("partial"), 0600); err != nil {
			return err
		}
		return errors.New("producer failed")
	}
	if err := produceFile(path, fail); err == nil {
		t.Fatal("expected producer error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial output published")
	}
	if err := produceFile(path, func(tmp string) error { return os.WriteFile(tmp, []byte("finished"), 0600) }); err != nil {
		t.Fatal(err)
	}
	if err := produceFile(path, func(string) error { t.Fatal("producer invoked for existing output"); return nil }); err == nil {
		t.Fatal("overwrite accepted")
	}
}

func TestRepoMapFindsGoSymbols(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "auth.go"), []byte("package auth\ntype Token string\nfunc Validate() {}\n"), 0600)
	os.Mkdir(filepath.Join(root, "node_modules"), 0700)
	os.WriteFile(filepath.Join(root, "node_modules", "hidden.go"), []byte("package hidden"), 0600)
	text, err := RepoMap(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "2: type Token") || !strings.Contains(text, "3: func Validate") || strings.Contains(text, "hidden.go") {
		t.Fatalf("map=%s", text)
	}
}

func TestExtensionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DispatchContext(ctx, "synthesize_speech", `{"text":"hello","path":"out.wav"}`); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestToolSchemasHaveArrayRequired(t *testing.T) {
	for _, tool := range Definitions() {
		var schema map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema["required"].([]any); !ok {
			t.Fatalf("%s has invalid required schema: %s", tool.Function.Name, tool.Function.Parameters)
		}
	}
}
