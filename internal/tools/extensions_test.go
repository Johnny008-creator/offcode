package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
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

func toolNames(defs []backend.Tool) map[string]bool {
	names := make(map[string]bool, len(defs))
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	return names
}

func TestExtensionGatingHidesUnconfiguredTools(t *testing.T) {
	emptyPath := t.TempDir()
	home := t.TempDir()
	t.Setenv("PATH", emptyPath)
	t.Setenv("HOME", home)
	t.Setenv("OFFCODE_WHISPER_MODEL", "")

	names := toolNames(Definitions())
	for _, gated := range []string{"generate_image", "generate_video", "generate_music", "export_document", "transcribe_audio", "synthesize_speech"} {
		if names[gated] {
			t.Errorf("%s offered with no dependency installed/configured", gated)
		}
	}
	for _, always := range []string{"read", "write", "repo_map", "git_summary"} {
		if !names[always] {
			t.Errorf("%s missing", always)
		}
	}
}

func TestExtensionGatingOffersConfiguredTools(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"pandoc", "espeak-ng"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)

	home := t.TempDir()
	t.Setenv("HOME", home)
	offcodeDir := filepath.Join(home, ".offcode")
	if err := os.MkdirAll(offcodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	workflow := filepath.Join(offcodeDir, "image.json")
	if err := os.WriteFile(workflow, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	mediaConfig := `{"workflows":{"image":{"file":"image.json"}}}`
	if err := os.WriteFile(filepath.Join(offcodeDir, "media.json"), []byte(mediaConfig), 0600); err != nil {
		t.Fatal(err)
	}

	names := toolNames(Definitions())
	for _, available := range []string{"export_document", "synthesize_speech", "generate_image"} {
		if !names[available] {
			t.Errorf("%s not offered despite dependency present", available)
		}
	}
	for _, unavailable := range []string{"generate_video", "generate_music", "transcribe_audio"} {
		if names[unavailable] {
			t.Errorf("%s offered without its workflow/model configured", unavailable)
		}
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
