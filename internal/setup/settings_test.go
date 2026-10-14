package setup

import (
	"os"
	"testing"
)

func TestPreserveEnvironment(t *testing.T) {
	t.Setenv("OLLAMA_VULKAN", "current")
	t.Setenv("OFFCODE_NUM_CTX", "16384")
	t.Setenv("HIP_VISIBLE_DEVICES", "")
	os.Unsetenv("HIP_VISIBLE_DEVICES")
	Apply(Settings{OllamaMode: "preserve", OllamaEnv: map[string]string{"OLLAMA_VULKAN": "saved", "HIP_VISIBLE_DEVICES": "0", "UNTRUSTED_SETUP_VAR": "bad"}})
	if os.Getenv("OLLAMA_VULKAN") != "current" || os.Getenv("HIP_VISIBLE_DEVICES") != "0" || os.Getenv("OFFCODE_NUM_CTX") != "16384" {
		t.Fatal("environment precedence broken")
	}
	if os.Getenv("UNTRUSTED_SETUP_VAR") != "" {
		t.Fatal("arbitrary environment injected")
	}
}
func TestSettingsRoundTrip(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	s, err := Load()
	if err != nil || s.Updates != "auto" || s.OllamaMode != "preserve" {
		t.Fatal(s, err)
	}
	s.Updates = "off"
	s.OllamaMode = "external"
	s.OllamaEnv = map[string]string{"OLLAMA_VULKAN": "1"}
	if err = Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.OllamaEnv["OLLAMA_VULKAN"] != "1" || got.Updates != "off" {
		t.Fatal(got, err)
	}
	s.Updates = "invalid"
	if Save(s) == nil {
		t.Fatal("invalid mode accepted")
	}
}

func isolateBackendEnv(t *testing.T) {
	t.Helper()
	for _, k := range append(append([]string{}, EnvironmentKeys...), "OFFCODE_NUM_CTX") {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestPreserveOnlyServer(t *testing.T) {
	isolateBackendEnv(t)
	s := Settings{OllamaMode: "preserve", Preserve: []string{"server"}, OllamaEnv: map[string]string{
		"OLLAMA_HOST": "127.0.0.1:12345", "OLLAMA_VULKAN": "1", "OLLAMA_MODELS": "old-model-dir", "OLLAMA_CONTEXT_LENGTH": "32768",
	}}
	Apply(s)
	if os.Getenv("OLLAMA_HOST") != "127.0.0.1:12345" {
		t.Fatal("selected server not restored")
	}
	for _, key := range []string{"OLLAMA_VULKAN", "OLLAMA_MODELS", "OLLAMA_CONTEXT_LENGTH", "OFFCODE_NUM_CTX"} {
		if _, ok := os.LookupEnv(key); ok {
			t.Fatalf("unchecked setting %s restored", key)
		}
	}
}

func TestCaptureDropsUncheckedPreviousValues(t *testing.T) {
	isolateBackendEnv(t)
	t.Setenv("OLLAMA_VULKAN", "current-gpu")
	t.Setenv("OLLAMA_MODELS", "current-model-dir")
	s := Settings{OllamaMode: "preserve", Preserve: []string{"server"}, OllamaEnv: map[string]string{"OLLAMA_HOST": "saved-host", "OLLAMA_VULKAN": "old-gpu"}}
	s.Capture()
	if len(s.OllamaEnv) != 1 || s.OllamaEnv["OLLAMA_HOST"] != "saved-host" {
		t.Fatal(s.OllamaEnv)
	}
	// Unchecking is not a destructive reset of the user's current environment.
	if os.Getenv("OLLAMA_VULKAN") != "current-gpu" {
		t.Fatal("modified system/process settings")
	}
	s.Preserve = []string{"gpu"}
	s.Capture()
	if len(s.OllamaEnv) != 1 || s.OllamaEnv["OLLAMA_VULKAN"] != "current-gpu" {
		t.Fatal(s.OllamaEnv)
	}
}

func TestPreserveMigrationAndExplicitNone(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	for _, mode := range []string{"preserve", "external", "auto"} {
		s := Settings{OllamaMode: mode, Updates: "off"}
		if s.Keeps("gpu") != (mode != "auto") {
			t.Fatal("legacy mode", mode)
		}
		s.Preserve = []string{}
		if err := Save(s); err != nil {
			t.Fatal(err)
		}
		got, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if got.Preserve == nil || got.Keeps("gpu") || got.Keeps("context") {
			t.Fatal("empty checkbox selection reverted", got)
		}
	}
}

func TestContextCheckboxIndependentOfGPU(t *testing.T) {
	isolateBackendEnv(t)
	Apply(Settings{OllamaMode: "preserve", Preserve: []string{"context"}, OllamaEnv: map[string]string{"OLLAMA_CONTEXT_LENGTH": "16384", "OLLAMA_VULKAN": "1"}})
	if os.Getenv("OFFCODE_NUM_CTX") != "0" || os.Getenv("OLLAMA_CONTEXT_LENGTH") != "16384" {
		t.Fatal("context not preserved")
	}
	if _, ok := os.LookupEnv("OLLAMA_VULKAN"); ok {
		t.Fatal("GPU unexpectedly preserved")
	}
}

func TestPreservationChoices(t *testing.T) {
	for _, bad := range []string{"", "server,typo", "server,", "all,gpu"} {
		if _, err := ParsePreserve(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
	all, err := ParsePreserve("all")
	if err != nil || len(all) != len(PreserveGroups) {
		t.Fatal(all, err)
	}
	none, err := ParsePreserve("none")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatal(none, err)
	}
	some, err := ParsePreserve("server,gpu,server")
	if err != nil || len(some) != 2 {
		t.Fatal(some, err)
	}
}

// Preserving context with nothing configured must not disable offcode's
// request context: Ollama's VRAM default (4096 on 8GB) truncated offcode's
// system prompt live on 2026-10-14.
func TestContextPreservedOnlyWhenConfigured(t *testing.T) {
	isolateBackendEnv(t)
	Apply(Settings{OllamaMode: "preserve"})
	if _, ok := os.LookupEnv("OFFCODE_NUM_CTX"); ok {
		t.Fatal("request context disabled although no server context is configured")
	}
	isolateBackendEnv(t)
	Apply(Settings{OllamaMode: "preserve", OllamaEnv: map[string]string{"OLLAMA_CONTEXT_LENGTH": "16384"}})
	if os.Getenv("OFFCODE_NUM_CTX") != "0" || os.Getenv("OLLAMA_CONTEXT_LENGTH") != "16384" {
		t.Fatal("configured server context not preserved")
	}
}
