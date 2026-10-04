// Package mcpclient connects offcode to external MCP (Model Context
// Protocol) servers as additional tool providers, configured by hand
// (never by the model itself — same trust-boundary stance as
// internal/hooks) via ~/.offcode/mcp.json and/or <cwd>/.offcode/mcp.json.
//
// Each connected server's tools are merged into the model's tool list
// with a "mcp_<server>_" name prefix, both to avoid colliding with the
// built-in tools and to make a tool's origin legible to the model (the
// same namespacing convention this very assistant's own MCP tools use).
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Config struct {
	Servers []ServerConfig `json:"servers"`
}

// Load merges the global config (~/.offcode/mcp.json) with a
// project-local one (<cwd>/.offcode/mcp.json), project servers appended
// after global. Returns an empty Config (not an error) if neither file
// exists — MCP is opt-in.
func Load(cwd string) (Config, error) {
	var cfg Config

	home, err := os.UserHomeDir()
	if err == nil {
		if global, ok, err := readFile(filepath.Join(home, ".offcode", "mcp.json")); err == nil && ok {
			cfg.Servers = append(cfg.Servers, global.Servers...)
		} else if err != nil {
			return cfg, err
		}
	}

	if local, ok, err := readFile(filepath.Join(cwd, ".offcode", "mcp.json")); err == nil && ok {
		cfg.Servers = append(cfg.Servers, local.Servers...)
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

const toolPrefix = "mcp_"

type connectedServer struct {
	name    string
	session *mcp.ClientSession
	tools   map[string]mcp.Tool // keyed by the tool's OWN (unprefixed) name
}

// Manager holds live connections to every MCP server that started
// successfully. A server that fails to start (command not found, bad
// config) is skipped with a warning rather than failing the whole app —
// matching the project's general stance that an optional integration's
// absence should degrade gracefully, not crash startup.
type Manager struct {
	mu      sync.Mutex
	servers []*connectedServer
}

// Connect starts every configured server and lists its tools. Returns
// the Manager (always non-nil, usable even with zero connected servers)
// and one warning string per server that failed, for the caller to
// display.
func Connect(ctx context.Context, cfg Config) (*Manager, []string) {
	m := &Manager{}
	var warnings []string

	for _, sc := range cfg.Servers {
		if sc.Name == "" || sc.Command == "" {
			warnings = append(warnings, "mcp server config missing name/command, skipped")
			continue
		}
		// context.Background(), deliberately not ctx: this process must
		// outlive the connect-phase timeout below and keep running for
		// the Manager's whole lifetime — it's only ever stopped via
		// session.Close() (see Manager.Close()), never by a context
		// deadline meant just to bound how long we wait for the initial
		// handshake.
		cmd := exec.CommandContext(context.Background(), sc.Command, sc.Args...)
		if len(sc.Env) > 0 {
			cmd.Env = os.Environ()
			for k, v := range sc.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}

		client := mcp.NewClient(&mcp.Implementation{Name: "offcode", Version: "0.1.0"}, nil)
		connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		session, err := client.Connect(connectCtx, &mcp.CommandTransport{Command: cmd}, nil)
		cancel()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("mcp server %q failed to start: %v", sc.Name, err))
			continue
		}

		listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		result, err := session.ListTools(listCtx, nil)
		cancel()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("mcp server %q connected but ListTools failed: %v", sc.Name, err))
			session.Close()
			continue
		}

		tools := make(map[string]mcp.Tool, len(result.Tools))
		for _, t := range result.Tools {
			tools[t.Name] = *t
		}
		m.servers = append(m.servers, &connectedServer{name: sc.Name, session: session, tools: tools})
	}

	return m, warnings
}

// Close disconnects every connected server. Best-effort — called on
// shutdown, errors here don't matter to the user.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		_ = s.session.Close()
	}
}

func prefixedName(serverName, toolName string) string {
	return toolPrefix + sanitize(serverName) + "_" + toolName
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, s)
}

// Tools returns every connected server's tools as backend.Tool
// definitions, named with the mcp_<server>_ prefix.
func (m *Manager) Tools() []backend.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []backend.Tool
	for _, s := range m.servers {
		for name, t := range s.tools {
			schema, err := json.Marshal(t.InputSchema)
			if err != nil || len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			out = append(out, backend.Tool{Type: "function", Function: backend.FunctionDef{
				Name:        prefixedName(s.name, name),
				Description: fmt.Sprintf("[via MCP server %q] %s", s.name, t.Description),
				Parameters:  schema,
			}})
		}
	}
	return out
}

// IsMCPTool reports whether name looks like one of this Manager's
// prefixed tool names, so callers can route dispatch without needing to
// try-and-fail against the real tool list first.
func IsMCPTool(name string) bool {
	return strings.HasPrefix(name, toolPrefix)
}

// Dispatch calls a prefixed tool name (as returned by Tools) against the
// server it belongs to.
func (m *Manager) Dispatch(ctx context.Context, name, argsJSON string) (string, error) {
	m.mu.Lock()
	var target *connectedServer
	var realName string
	for _, s := range m.servers {
		prefix := toolPrefix + sanitize(s.name) + "_"
		if strings.HasPrefix(name, prefix) {
			candidate := strings.TrimPrefix(name, prefix)
			if _, ok := s.tools[candidate]; ok {
				target = s
				realName = candidate
				break
			}
		}
	}
	m.mu.Unlock()

	if target == nil {
		return "", fmt.Errorf("unknown MCP tool: %q", name)
	}

	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments JSON: %w", err)
		}
	}

	result, err := target.session.CallTool(ctx, &mcp.CallToolParams{Name: realName, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("mcp call to %s/%s: %w", target.name, realName, err)
	}

	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
			sb.WriteString("\n")
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		text = "(tool returned no text content)"
	}
	if result.IsError {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}
