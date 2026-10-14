package config

import "testing"

func TestOllamaHostResolution(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"127.0.0.1:12345", "http://127.0.0.1:12345/v1"},
		{"0.0.0.0:11434", "http://localhost:11434/v1"},
		{"http://[::]:11434", "http://localhost:11434/v1"},
		{"http://[::1]:1234", "http://[::1]:1234/v1"},
		{"gpu-server", "http://gpu-server:11434/v1"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			t.Setenv("OLLAMA_HOST", tc.host)
			got, err := ResolveBaseURL("ollama", "")
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
			got, err = ResolveBaseURL("ollama", "http://explicit/v1")
			if err != nil || got != "http://explicit/v1" {
				t.Fatal(got, err)
			}
			got, err = ResolveBaseURL("lmstudio", "")
			if err != nil || got != "http://localhost:1234/v1" {
				t.Fatal(got, err)
			}
		})
	}
}
