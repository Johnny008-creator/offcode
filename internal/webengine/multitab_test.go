package webengine

import (
	"context"
	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"testing"
	"time"
)

func TestStaleConfirmationCannotApproveNextTool(t *testing.T) {
	s := &Session{sessionID: "session", status: "generating", turnCtx: context.Background()}
	wait := func() (string, <-chan bool) {
		done := make(chan bool, 1)
		go func() {
			done <- s.waitForConfirm(backend.ToolCall{ID: "reused-model-id", Function: backend.FunctionCall{Name: "write", Arguments: `{"path":"test.txt","content":"ok"}`}})
		}()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if c := s.Snapshot().Confirm; c != nil {
				return c.ID, done
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("confirmation missing")
		return "", done
	}
	first, done := wait()
	if !s.Confirm(true, first) || !<-done {
		t.Fatal("first confirmation failed")
	}
	second, done := wait()
	if first == second {
		t.Fatal("request IDs reused")
	}
	if s.Confirm(true, first) {
		t.Fatal("stale tab approved a different tool")
	}
	if !s.Confirm(false, second) || <-done {
		t.Fatal("second confirmation failed")
	}
}

func TestStaleSessionMutationsAndArtifacts(t *testing.T) {
	s := &Session{sessionID: "new", status: "idle", cfg: config.Config{Mode: "default"}, blocks: []Block{{artifactPath: "new.png"}}}
	if s.Submit("old draft", "old") || s.SubmitDisplay("old", "old", "old") || s.StartNew("old") || s.SwitchTo("other", "old") || s.SetMode("auto", "old") {
		t.Fatal("stale tab changed session")
	}
	if _, ok := s.ArtifactPath(0, "old"); ok {
		t.Fatal("stale artifact resolved into a different session")
	}
	if path, ok := s.ArtifactPath(0, "new"); !ok || path != "new.png" {
		t.Fatal("current artifact missing")
	}
}

func TestStaleAnswerCannotConsumeNewQuestion(t *testing.T) {
	s := &Session{pendingAskID: "second", askCh: make(chan string, 1)}
	if s.AnswerAsk("wrong", "first") {
		t.Fatal("stale answer accepted")
	}
	ch := s.askCh
	if !s.AnswerAsk("right", "second") {
		t.Fatal("current answer rejected")
	}
	if answer := <-ch; answer != "right" {
		t.Fatal(answer)
	}
}
