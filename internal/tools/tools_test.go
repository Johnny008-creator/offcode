package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteRejectsCorruptedContentKey covers a real failure found live in
// the first 5-tier Ollama campaign (2026-10-10, see docs/DECISIONS.md):
// Ember (qwen3.5:4b)'s chat template mangled the "content" parameter tag
// into an empty-string key, and offcode silently wrote an empty file
// instead of surfacing that something went wrong.
func TestWriteRejectsCorruptedContentKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.html")
	args, err := json.Marshal(map[string]string{"path": path, "": "content>\n<!DOCTYPE html>"})
	if err != nil {
		t.Fatal(err)
	}
	argsJSON := string(args)
	_, err = Dispatch("write", argsJSON)
	if err == nil {
		t.Fatal("expected an error for a missing content key")
	}
	if !strings.Contains(err.Error(), `"content"`) {
		t.Fatalf("error should mention the missing content key, got: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("corrupted call should not have created a file")
	}
}

// TestWriteAllowsDeliberateEmptyFile ensures the fix above doesn't break
// the legitimate case: a model asking to create an empty file by simply
// omitting "content" (not sending a corrupted stray key).
func TestWriteAllowsDeliberateEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	argsJSON := string(args)
	if _, err = Dispatch("write", argsJSON); err != nil {
		t.Fatalf("expected a deliberate empty-content write to succeed, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("expected an empty file, got %q", data)
	}
}
