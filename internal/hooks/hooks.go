// Package hooks runs user-defined shell commands around tool calls —
// pre_tool_use (can deny a call before it runs) and post_tool_use (runs
// after, non-blocking, for side effects like notifications or linting).
//
// Deliberately configured only by hand-edited JSON files, never by a
// tool the model can call: hooks are a human-set policy/trust boundary
// (e.g. "never let anything touch /etc" or "run a linter after every
// write"), and letting the model grant itself new hook rules would
// defeat that boundary's whole point.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Event string

const (
	PreToolUse  Event = "pre_tool_use"
	PostToolUse Event = "post_tool_use"
)

type Hook struct {
	Event   Event  `json:"event"`
	Matcher string `json:"matcher"` // exact tool name, or "*" for every tool
	Command string `json:"command"` // run via `sh -c` (or `cmd /C` on Windows)
}

type Config struct {
	Hooks []Hook `json:"hooks"`
}

const hookTimeout = 10 * time.Second

// Load merges the global hooks file (~/.offcode/hooks.json) with a
// project-local one (<cwd>/.offcode/hooks.json, if present) — project
// hooks run after global ones. Returns an empty Config (not an error) if
// neither file exists, the common case.
func Load(cwd string) (Config, error) {
	var cfg Config

	home, err := os.UserHomeDir()
	if err == nil {
		if global, ok, err := readFile(filepath.Join(home, ".offcode", "hooks.json")); err == nil && ok {
			cfg.Hooks = append(cfg.Hooks, global.Hooks...)
		} else if err != nil {
			return cfg, err
		}
	}

	if local, ok, err := readFile(filepath.Join(cwd, ".offcode", "hooks.json")); err == nil && ok {
		cfg.Hooks = append(cfg.Hooks, local.Hooks...)
	} else if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func readFile(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, true, nil
}

func matches(matcher, toolName string) bool {
	return matcher == "*" || matcher == toolName
}

func run(command, toolName, argsJSON, event, extra string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Env = append(os.Environ(),
		"OFFCODE_EVENT="+event,
		"OFFCODE_TOOL_NAME="+toolName,
		"OFFCODE_TOOL_ARGS="+argsJSON,
	)
	if extra != "" {
		cmd.Env = append(cmd.Env, "OFFCODE_TOOL_RESULT="+extra)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// PreResult is whether any matching pre_tool_use hook denied the call.
type PreResult struct {
	Denied bool
	Reason string
}

// RunPre runs every pre_tool_use hook matching toolName, in order. The
// first one to exit non-zero denies the call — its stderr (or stdout, if
// stderr is empty) becomes the denial reason shown to the model.
func RunPre(cfg Config, toolName, argsJSON string) PreResult {
	for _, h := range cfg.Hooks {
		if h.Event != PreToolUse || !matches(h.Matcher, toolName) {
			continue
		}
		stdout, stderr, err := run(h.Command, toolName, argsJSON, string(PreToolUse), "")
		if err != nil {
			reason := stderr
			if reason == "" {
				reason = stdout
			}
			if reason == "" {
				reason = err.Error()
			}
			return PreResult{Denied: true, Reason: reason}
		}
	}
	return PreResult{}
}

// RunPost runs every post_tool_use hook matching toolName. Never blocks
// the tool result (the tool already ran) — a hook that fails just
// produces a short warning note for display, so a broken hook script is
// visible without derailing the agent loop.
func RunPost(cfg Config, toolName, argsJSON, result string) []string {
	var warnings []string
	for _, h := range cfg.Hooks {
		if h.Event != PostToolUse || !matches(h.Matcher, toolName) {
			continue
		}
		_, stderr, err := run(h.Command, toolName, argsJSON, string(PostToolUse), result)
		if err != nil {
			msg := stderr
			if msg == "" {
				msg = err.Error()
			}
			warnings = append(warnings, fmt.Sprintf("post_tool_use hook for %q failed: %s", h.Matcher, msg))
		}
	}
	return warnings
}
