package hooks

import (
	"runtime"
	"testing"
)

func failingCommand(message string) string {
	if runtime.GOOS == "windows" {
		return "echo " + message + " 1>&2 & exit /b 1"
	}
	return "echo " + message + " >&2; exit 1"
}

func TestRunPreAllowsWhenNoHooksMatch(t *testing.T) {
	cfg := Config{}
	res := RunPre(cfg, "bash", `{"command":"ls"}`)
	if res.Denied {
		t.Error("expected no denial with an empty config")
	}
}

func TestRunPreDeniesOnNonZeroExit(t *testing.T) {
	cfg := Config{Hooks: []Hook{
		{Event: PreToolUse, Matcher: "bash", Command: failingCommand("nope")},
	}}
	res := RunPre(cfg, "bash", `{"command":"rm -rf /"}`)
	if !res.Denied {
		t.Fatal("expected the hook's non-zero exit to deny the call")
	}
	if res.Reason != "nope" {
		t.Errorf("expected reason %q, got %q", "nope", res.Reason)
	}
}

func TestRunPreIgnoresNonMatchingTool(t *testing.T) {
	cfg := Config{Hooks: []Hook{
		{Event: PreToolUse, Matcher: "write", Command: "exit 1"},
	}}
	res := RunPre(cfg, "bash", "{}")
	if res.Denied {
		t.Error("hook for a different tool name should not have fired")
	}
}

func TestRunPreWildcardMatchesEveryTool(t *testing.T) {
	cfg := Config{Hooks: []Hook{
		{Event: PreToolUse, Matcher: "*", Command: "exit 1"},
	}}
	res := RunPre(cfg, "anything", "{}")
	if !res.Denied {
		t.Error("wildcard matcher should match every tool")
	}
}

func TestRunPostReturnsWarningOnFailure(t *testing.T) {
	cfg := Config{Hooks: []Hook{
		{Event: PostToolUse, Matcher: "write", Command: failingCommand("boom")},
	}}
	warnings := RunPost(cfg, "write", "{}", "wrote 10 bytes")
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
}

func TestRunPostSilentOnSuccess(t *testing.T) {
	cfg := Config{Hooks: []Hook{
		{Event: PostToolUse, Matcher: "*", Command: "exit 0"},
	}}
	warnings := RunPost(cfg, "write", "{}", "ok")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}
