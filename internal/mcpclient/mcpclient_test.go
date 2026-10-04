package mcpclient

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestConnectRealFilesystemServer is a real integration test against the
// official @modelcontextprotocol/server-filesystem via npx — skipped if
// npx/network isn't available, since it needs to fetch/run a real
// subprocess, not a mock.
func TestConnectRealFilesystemServer(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/sample.txt", []byte("hello mcp world"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{Servers: []ServerConfig{
		{Name: "fs", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", dir}},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m, warnings := Connect(ctx, cfg)
	defer m.Close()
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings connecting: %v", warnings)
	}

	tools := m.Tools()
	if len(tools) == 0 {
		t.Fatal("expected at least one tool from the filesystem server")
	}

	var readToolName string
	for _, tl := range tools {
		if strings.Contains(tl.Function.Name, "read_text_file") || strings.Contains(tl.Function.Name, "read_file") {
			readToolName = tl.Function.Name
			break
		}
	}
	if readToolName == "" {
		var names []string
		for _, tl := range tools {
			names = append(names, tl.Function.Name)
		}
		t.Fatalf("no read-file-like tool found among: %v", names)
	}
	if !IsMCPTool(readToolName) {
		t.Errorf("expected %q to be recognized as an MCP tool", readToolName)
	}

	result, err := m.Dispatch(ctx, readToolName, `{"path":"`+dir+`/sample.txt"}`)
	if err != nil {
		t.Fatalf("Dispatch(%s): %v", readToolName, err)
	}
	if !strings.Contains(result, "hello mcp world") {
		t.Errorf("expected file content in result, got: %q", result)
	}
}

// TestServerSurvivesConnectContextExpiry is a regression test for a real
// bug caught before it shipped: Connect's subprocess must not be tied to
// the same context used to bound the initial handshake timeout, or the
// MCP server dies the moment that short-lived context is canceled —
// breaking every tool call for the rest of the session.
func TestServerSurvivesConnectContextExpiry(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/sample.txt", []byte("still alive"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{Servers: []ServerConfig{
		{Name: "fs", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", dir}},
	}}

	// A short-lived context, canceled right after Connect returns — the
	// same shape as the TUI's real connectMCPCmd.
	connectCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	m, warnings := Connect(connectCtx, cfg)
	cancel() // simulate the caller's context going away immediately
	defer m.Close()
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	time.Sleep(200 * time.Millisecond) // give a killed process time to actually die

	tools := m.Tools()
	var readToolName string
	for _, tl := range tools {
		if strings.Contains(tl.Function.Name, "read_text_file") || strings.Contains(tl.Function.Name, "read_file") {
			readToolName = tl.Function.Name
		}
	}
	if readToolName == "" {
		t.Fatal("no read tool found")
	}

	// Use a fresh, independent context for the actual call — proving the
	// server process is still running despite connectCtx being long dead.
	callCtx, callCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer callCancel()
	result, err := m.Dispatch(callCtx, readToolName, `{"path":"`+dir+`/sample.txt"}`)
	if err != nil {
		t.Fatalf("server died after connect context expired: %v", err)
	}
	if !strings.Contains(result, "still alive") {
		t.Errorf("expected file content, got: %q", result)
	}
}
