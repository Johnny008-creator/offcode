package webengine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func TestDiffPreview(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"path": "test.go", "old_string": "keep\nold\nend", "new_string": "keep\nnew\nend"})
	preview := diffPreview(backend.ToolCall{Function: backend.FunctionCall{Name: "edit", Arguments: string(data)}})
	if !strings.Contains(preview, "- old") || !strings.Contains(preview, "+ new") || strings.Contains(preview, "keep") {
		t.Fatalf("preview=%s", preview)
	}
}

func TestArtifactRegistrationRequiresProducer(t *testing.T) {
	for _, name := range []string{"read", "generate_image"} {
		b := artifactBlock(backend.ToolCall{Function: backend.FunctionCall{Name: name}}, `{"artifact":"out.png"}`, false, 0)
		s := &Session{blocks: []Block{b}}
		_, ok := s.ArtifactPath(0)
		if ok != (name == "generate_image") {
			t.Fatalf("tool %s registered=%v", name, ok)
		}
		if _, ok := s.ArtifactPath(1); ok {
			t.Fatal("out of range artifact accepted")
		}
	}
}

func TestStopUnblocksConfirmation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Session{turnCtx: ctx, cancelFn: cancel}
	done := make(chan bool, 1)
	go func() { done <- s.waitForConfirm(backend.ToolCall{Function: backend.FunctionCall{Name: "bash"}}) }()
	s.Stop()
	select {
	case allowed := <-done:
		if allowed {
			t.Fatal("stop approved a tool")
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not unblock confirmation")
	}
}
