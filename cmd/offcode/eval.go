package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/webengine"
)

// runEvalCase runs a single prompt through the actual web agent engine.
// The outer runner supplies a fresh cwd and an isolated HOME for every case.
func runEvalCase(args []string) {
	fs := flag.NewFlagSet("offcode eval-case", flag.ExitOnError)
	backendName := fs.String("backend", "ollama", "ollama or lmstudio")
	baseURL := fs.String("base-url", "", "backend endpoint")
	model := fs.String("model", "", "backend model tag")
	promptPath := fs.String("prompt-file", "", "prompt file")
	timeout := fs.Duration("timeout", 10*time.Minute, "per-case timeout")
	fs.Parse(args)
	if strings.TrimSpace(*model) == "" {
		failExtra(fmt.Errorf("-model is required"))
	}
	cfg, err := config.Resolve(*backendName, *baseURL, *model, "default")
	if err != nil {
		failExtra(err)
	}
	prompt, err := os.ReadFile(*promptPath)
	if err != nil {
		failExtra(err)
	}
	client := backend.NewClient(cfg.BaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = client.ListModels(ctx, cfg.Backend)
	cancel()
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "blocked", "error": err.Error()})
		return
	}
	s := webengine.NewSession(cfg)
	if !s.Submit(string(prompt)) {
		failExtra(fmt.Errorf("cannot submit prompt"))
	}
	deadline := time.NewTimer(*timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		state := s.Snapshot()
		if state.Confirm != nil {
			allow := false
			switch state.Confirm.ToolName {
			case "write", "edit":
				var a struct{ Path string }
				_ = json.Unmarshal([]byte(state.Confirm.ToolArgs), &a)
				allow = evalWriteAllowed(a.Path)
			case "bash":
				var a struct{ Command string }
				_ = json.Unmarshal([]byte(state.Confirm.ToolArgs), &a)
				switch strings.TrimSpace(a.Command) {
				case "ls -lS", "ls -laS", "ls -S", "dir /o-s":
					allow = true
				}
			}
			s.Confirm(allow)
		}
		if state.Ask != nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "asked_user", "state": state})
			s.Stop()
			return
		}
		if state.Status == "idle" {
			status := "completed"
			for _, b := range state.Blocks {
				if b.IsError || b.Kind == webengine.BlockError {
					status = "errors"
				}
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": status, "state": state})
			return
		}
		select {
		case <-deadline.C:
			s.Stop()
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "timeout", "state": state})
			return
		case <-ticker.C:
		}
	}
}

// evalWriteAllowed approves writes that stay inside the case's working
// directory, whether the model names the file relatively or by its
// absolute path (small models often echo the cwd from the system prompt).
func evalWriteAllowed(path string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return false
		}
		path = rel
	}
	return filepath.IsLocal(path) && !strings.HasPrefix(filepath.Clean(path), ".offcode")
}
