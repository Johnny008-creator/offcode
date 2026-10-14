package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Johnny008-creator/offcode/internal/config"
)

func TestOllamaReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if !ollamaReachable(srv.URL) {
		t.Fatal("expected reachable server to report true")
	}
	if ollamaReachable("http://127.0.0.1:1") {
		t.Fatal("expected an address nothing listens on to report false")
	}
}

func TestEnsureOllamaRunningNoopForNonOllamaBackend(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	start := time.Now()
	ensureOllamaRunning(config.Config{Backend: "lmstudio", BaseURL: "http://127.0.0.1:1/v1"})
	if time.Since(start) > time.Second {
		t.Fatal("should return immediately for a non-ollama backend")
	}
}

func TestEnsureOllamaRunningNoopWhenAlreadyReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	start := time.Now()
	ensureOllamaRunning(config.Config{Backend: "ollama", BaseURL: srv.URL + "/v1"})
	if time.Since(start) > time.Second {
		t.Fatal("should return immediately once the backend answers")
	}
}

func TestEnsureOllamaRunningNoopWhenBinaryMissing(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	start := time.Now()
	ensureOllamaRunning(config.Config{Backend: "ollama", BaseURL: "http://127.0.0.1:1/v1"})
	if time.Since(start) > time.Second {
		t.Fatal("should return immediately when the ollama binary isn't on PATH, not wait out the poll loop")
	}
}

func TestEnsureOllamaRunningStartsAFoundBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture; process replacement has a portable test in internal/updater")
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "started")
	script := "#!/bin/sh\n: > '" + marker + "'\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "ollama"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	// ensureOllamaRunning itself polls for up to 10s before giving up on an
	// unreachable address — run it in the background and only wait for the
	// marker file, so this test doesn't need to sit through that timeout.
	go ensureOllamaRunning(config.Config{Backend: "ollama", BaseURL: "http://127.0.0.1:1/v1"})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected the found `ollama` binary to have been started")
}
