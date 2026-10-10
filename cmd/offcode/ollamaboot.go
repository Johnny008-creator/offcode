package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/config"
)

// ensureOllamaRunning best-effort auto-starts a local `ollama serve` when
// the configured backend is Ollama but nothing answers yet. Found live:
// a user expects launching offcode to just work, not to separately know
// Ollama needs starting first — its systemd unit ships disabled by
// default on Arch, and the desktop app isn't always running, so offcode
// just failed every tool call with a raw "connection refused" instead.
// Silently does nothing if the ollama binary isn't on PATH, the
// configured backend isn't Ollama, or something is already listening;
// never blocks more than a few seconds either way, and never treats
// failure to start it as fatal — the existing connection-refused error
// still surfaces normally if this doesn't help.
func ensureOllamaRunning(cfg config.Config) {
	if cfg.Backend != "ollama" {
		return
	}
	root := strings.TrimSuffix(cfg.BaseURL, "/v1")
	if ollamaReachable(root) {
		return
	}
	path, err := exec.LookPath("ollama")
	if err != nil {
		return
	}
	cmd := exec.Command(path, "serve")
	detachOllamaProcess(cmd)
	if err := cmd.Start(); err != nil {
		return
	}
	fmt.Fprintln(os.Stderr, "offcode: Ollama wasn't running — starting it now...")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ollamaReachable(root) {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "offcode: started `ollama serve` but it isn't answering yet — continuing anyway")
}

func ollamaReachable(root string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(root + "/api/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
