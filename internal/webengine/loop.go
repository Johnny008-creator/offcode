package webengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/agent"
	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/memory"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/skills"
	"github.com/Johnny008-creator/offcode/internal/tools"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
)

// runTurn drives the agent loop to completion (a final answer, an error,
// or a cancellation), pausing synchronously on tool confirmation and
// ask_user via blocking channel receives. It always leaves the session in
// status "idle" when it returns, and always runs in its own goroutine
// (started by Submit/SubmitDisplay).
func (s *Session) runTurn() {
	for {
		msg, usage, turnStart, err := s.streamOnce()
		if err != nil {
			s.mu.Lock()
			s.status = "idle"
			s.activity = ""
			s.mu.Unlock()
			if errors.Is(err, context.Canceled) {
				s.pushBlock(Block{Kind: BlockSystem, Text: "( interrupted )"})
			} else {
				s.pushBlock(Block{Kind: BlockError, Text: err.Error(), IsError: true})
			}
			s.saveToDiskLocked()
			s.notifyOnly()
			return
		}

		if usage.CompletionTokens > 0 {
			s.mu.Lock()
			switch {
			case usage.EvalDurationNs > 0:
				s.tokensPerSec = float64(usage.CompletionTokens) / (float64(usage.EvalDurationNs) / 1e9)
			case time.Since(turnStart).Seconds() > 0:
				s.tokensPerSec = float64(usage.CompletionTokens) / time.Since(turnStart).Seconds()
			}
			model := s.cfg.Model
			s.mu.Unlock()
			recordUsage(model, usage, time.Since(turnStart))
		}

		if len(msg.ToolCalls) == 0 {
			if name, argsJSON, ok := agent.ExtractPseudoToolCall(msg.Content); ok {
				msg.ToolCalls = []backend.ToolCall{{
					ID:       fmt.Sprintf("pseudo-%d", s.toolIterations),
					Type:     "function",
					Function: backend.FunctionCall{Name: name, Arguments: argsJSON},
				}}
				msg.Content = ""
			}
		}

		s.mu.Lock()
		s.messages = append(s.messages, msg)
		if msg.Content != "" {
			s.blocks = append(s.blocks, Block{Kind: BlockAssistant, Text: msg.Content})
		}
		s.mu.Unlock()

		if len(msg.ToolCalls) == 0 {
			if incomplete := s.incompletePlanSteps(); len(incomplete) > 0 && s.planNudges < maxPlanNudges {
				s.planNudges++
				s.pushBlock(Block{Kind: BlockSystem, Text: "( plan has unfinished steps — asking the model to confirm or continue )"})
				s.mu.Lock()
				s.messages = append(s.messages, backend.Message{
					Role: "user",
					Content: fmt.Sprintf(
						"Your plan still shows these steps as not done: %s. If you actually finished them, call plan_update for each one now. If not, finish them before answering.",
						strings.Join(incomplete, "; "),
					),
				})
				s.mu.Unlock()
				continue
			}
			s.mu.Lock()
			s.status = "idle"
			s.activity = ""
			s.mu.Unlock()
			s.saveToDiskLocked()
			s.notifyOnly()
			return
		}

		s.toolIterations++
		if s.toolIterations > maxToolIterationsPerTurn {
			s.pushBlock(Block{Kind: BlockError, Text: fmt.Sprintf("stopped after %d tool iterations without a final answer", maxToolIterationsPerTurn), IsError: true})
			s.mu.Lock()
			s.status = "idle"
			s.activity = ""
			s.mu.Unlock()
			s.saveToDiskLocked()
			return
		}

		for _, tc := range msg.ToolCalls {
			s.runToolCall(tc)
		}
		// loop back for another generation round
	}
}

// incompletePlanSteps must NOT be called with s.mu held.
func (s *Session) incompletePlanSteps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for i, p := range s.plan {
		if p.Status != stepDone && p.Status != stepSkipped {
			out = append(out, fmt.Sprintf("%d: %s", i, p.Text))
		}
	}
	return out
}

// streamOnce runs exactly one model request to completion (or error),
// streaming content/reasoning deltas into the transcript live.
func (s *Session) streamOnce() (backend.Message, backend.Usage, time.Time, error) {
	s.mu.Lock()
	model := s.cfg.Model
	backendKind := s.cfg.Backend
	think := s.think
	vision := s.visionCapable
	msgs := append([]backend.Message(nil), s.messages...)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.activity = "thinking"
	s.mu.Unlock()
	defer cancel()

	req := backend.ChatRequest{Model: model, Messages: msgs, Tools: toolsForModel(model, vision)}

	var ch <-chan backend.StreamEvent
	if backendKind == "ollama" {
		ch = s.client.StreamChatNative(ctx, req, think)
	} else {
		ch = s.client.StreamChatCompletion(ctx, req)
	}

	turnStart := time.Now()
	assistantBlockIdx := -1
	reasoningBlockIdx := -1

	for ev := range ch {
		switch ev.Kind {
		case backend.EventContent:
			s.mu.Lock()
			if assistantBlockIdx == -1 {
				s.blocks = append(s.blocks, Block{Kind: BlockAssistant})
				assistantBlockIdx = len(s.blocks) - 1
			}
			s.blocks[assistantBlockIdx].Text += ev.ContentDelta
			s.activity = "generating"
			s.notify()
			s.mu.Unlock()

		case backend.EventReasoning:
			s.mu.Lock()
			if reasoningBlockIdx == -1 {
				s.blocks = append(s.blocks, Block{Kind: BlockReasoning})
				reasoningBlockIdx = len(s.blocks) - 1
			}
			s.blocks[reasoningBlockIdx].Text += ev.ContentDelta
			s.activity = "thinking"
			s.notify()
			s.mu.Unlock()

		case backend.EventToolCallStart:
			s.mu.Lock()
			s.activity = "using tool: " + ev.ToolCallName
			s.notify()
			s.mu.Unlock()

		case backend.EventError:
			// The streaming assistant block (if any) stays in the transcript
			// as-is — it's a genuine partial response, not noise.
			return backend.Message{}, backend.Usage{}, turnStart, ev.Err

		case backend.EventDone:
			// Drop the now-finalized streaming assistant block: the caller
			// appends the authoritative version (ev.Message.Content) itself,
			// so remove our incremental copy to avoid a duplicate. Tool-only
			// messages never got an assistant block in the first place.
			if assistantBlockIdx != -1 {
				s.mu.Lock()
				s.blocks = append(s.blocks[:assistantBlockIdx], s.blocks[assistantBlockIdx+1:]...)
				s.mu.Unlock()
			}
			return ev.Message, ev.Usage, turnStart, nil
		}
	}

	return backend.Message{}, backend.Usage{}, turnStart, fmt.Errorf("stream closed unexpectedly")
}

// runToolCall executes (or synchronously pauses for confirmation / a user
// answer on) a single tool call, appending its result to the transcript.
func (s *Session) runToolCall(tc backend.ToolCall) {
	s.pushBlock(Block{Kind: BlockToolCall, ToolName: tc.Function.Name, ToolArgs: tc.Function.Arguments})

	switch tc.Function.Name {
	case "plan_set":
		s.handlePlanSet(tc)
		return
	case "plan_update":
		s.handlePlanUpdate(tc)
		return
	case "ask_user":
		s.handleAskUser(tc)
		return
	case "remember":
		s.handleRemember(tc)
		return
	case "load_skill":
		s.handleLoadSkill(tc)
		return
	case "view_image":
		s.handleViewImage(tc)
		return
	}

	s.mu.Lock()
	callKey := tc.Function.Name + "\x00" + tc.Function.Arguments
	if callKey == s.lastFailedCall && s.repeatFailCount >= 2 {
		s.mu.Unlock()
		s.recordToolResult(tc, fmt.Sprintf(
			"DENIED: this exact %s call with identical arguments has already failed %d times in a row — refusing to run it again verbatim, since it will just fail the same way. Change the arguments, try a different tool, or explain the problem to the user instead of repeating this call.",
			tc.Function.Name, s.repeatFailCount), true)
		return
	}
	mode := s.cfg.Mode
	s.mu.Unlock()

	decision := permission.Check(mode, tc.Function.Name, tc.Function.Arguments)

	if tc.Function.Name == "write" && !decision.Denied {
		path := extractPath(tc.Function.Arguments)
		s.mu.Lock()
		read := s.readPaths[path]
		s.mu.Unlock()
		if path != "" && !read {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				decision.NeedsConfirm = true
				s.pushBlock(Block{Kind: BlockSystem, Text: fmt.Sprintf(
					"warning: %s exists and hasn't been read in this session — forcing confirmation even in auto mode, since overwriting it would discard content the model hasn't seen", path)})
			}
		}
	}

	if decision.Denied {
		s.recordToolResult(tc, "DENIED: "+decision.DenyReason, true)
		return
	}

	if decision.NeedsConfirm {
		allow := s.waitForConfirm(tc)
		if !allow {
			s.recordToolResult(tc, "DENIED: user declined confirmation", true)
			return
		}
	}

	s.mu.Lock()
	s.status = "generating"
	s.activity = "running tool: " + tc.Function.Name
	s.notify()
	s.mu.Unlock()

	out, err := tools.Dispatch(tc.Function.Name, tc.Function.Arguments)
	if err != nil {
		content := "error: " + err.Error()
		if out != "" {
			content = fmt.Sprintf("error: %v\noutput:\n%s", err, out)
		}
		s.recordToolResult(tc, content, true)
		return
	}
	if tc.Function.Name == "read" || tc.Function.Name == "write" {
		if path := extractPath(tc.Function.Arguments); path != "" {
			s.mu.Lock()
			s.readPaths[path] = true
			s.mu.Unlock()
		}
	}
	s.recordToolResult(tc, out, false)
}

func (s *Session) waitForConfirm(tc backend.ToolCall) bool {
	ch := make(chan bool, 1)
	s.mu.Lock()
	s.pendingConfirmTC = &tc
	s.confirmCh = ch
	s.status = "confirming"
	s.activity = "awaiting confirmation"
	s.notify()
	s.mu.Unlock()

	allow := <-ch

	s.mu.Lock()
	s.pendingConfirmTC = nil
	s.status = "generating"
	s.mu.Unlock()
	return allow
}

func (s *Session) handleAskUser(tc backend.ToolCall) {
	var a struct {
		Question         string   `json:"question"`
		Options          []string `json:"options"`
		RecommendedIndex *int     `json:"recommended_index"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || a.Question == "" || len(a.Options) < 2 {
		s.recordToolResult(tc, "error: ask_user requires a question and at least 2 options", true)
		return
	}

	ch := make(chan string, 1)
	rec := -1
	if a.RecommendedIndex != nil {
		rec = *a.RecommendedIndex
	}
	s.mu.Lock()
	s.pendingAskTC = &tc
	s.pendingAskQuestion = a.Question
	s.pendingAskOptions = a.Options
	s.pendingAskRecommended = rec
	s.askCh = ch
	s.status = "asking"
	s.activity = "waiting for your answer"
	s.notify()
	s.mu.Unlock()

	answer := <-ch

	s.mu.Lock()
	s.pendingAskTC = nil
	s.status = "generating"
	s.mu.Unlock()

	s.recordToolResult(tc, answer, false)
}

func (s *Session) handlePlanSet(tc backend.ToolCall) {
	var a struct {
		Steps []string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || len(a.Steps) == 0 {
		s.recordToolResult(tc, "error: plan_set requires a non-empty \"steps\" array of strings", true)
		return
	}
	s.mu.Lock()
	s.plan = make([]planStep, len(a.Steps))
	for i, st := range a.Steps {
		s.plan[i] = planStep{Text: st}
	}
	s.plan[0].Status = stepInProgress
	s.mu.Unlock()
	s.recordToolResult(tc, fmt.Sprintf(
		"plan set with %d steps. Step 0 (%q) is marked in_progress automatically. "+
			"When you finish a step, call plan_update(index, \"done\") — the next pending step "+
			"will automatically be marked in_progress, so you don't need a separate call for that.",
		len(a.Steps), a.Steps[0],
	), false)
}

func (s *Session) handlePlanUpdate(tc backend.ToolCall) {
	var a struct {
		Index  int    `json:"index"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil {
		s.recordToolResult(tc, "error: invalid plan_update arguments", true)
		return
	}

	s.mu.Lock()
	if a.Index < 0 || a.Index >= len(s.plan) {
		n := len(s.plan)
		s.mu.Unlock()
		s.recordToolResult(tc, fmt.Sprintf("error: step index %d out of range (plan has %d steps, 0-indexed)", a.Index, n), true)
		return
	}
	status, ok := parsePlanStatus(a.Status)
	if !ok {
		s.mu.Unlock()
		s.recordToolResult(tc, "error: status must be one of: pending, in_progress, done, skipped", true)
		return
	}
	s.plan[a.Index].Status = status
	result := fmt.Sprintf("step %d marked %s.", a.Index, a.Status)

	if status == stepDone || status == stepSkipped {
		nextIdx := -1
		for i := range s.plan {
			if s.plan[i].Status == stepPending {
				nextIdx = i
				break
			}
		}
		switch {
		case nextIdx >= 0:
			s.plan[nextIdx].Status = stepInProgress
			result += fmt.Sprintf(" Step %d (%q) is now marked in_progress automatically.", nextIdx, s.plan[nextIdx].Text)
		default:
			allDone := true
			for _, p := range s.plan {
				if p.Status != stepDone && p.Status != stepSkipped {
					allDone = false
					break
				}
			}
			if allDone {
				result += " All steps are now done or skipped — give the user your closing summary of what you actually did."
			}
		}
	}
	s.mu.Unlock()

	s.recordToolResult(tc, result, false)
}

func (s *Session) handleRemember(tc backend.ToolCall) {
	var a struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Note) == "" {
		s.recordToolResult(tc, "error: remember requires a non-empty \"note\" string", true)
		return
	}
	if err := memory.Append(a.Note); err != nil {
		s.recordToolResult(tc, "error: couldn't save note: "+err.Error(), true)
		return
	}
	s.pushBlock(Block{Kind: BlockSystem, Text: "remembered: " + a.Note})
	s.recordToolResult(tc, "saved to persistent memory — it will be loaded into the system prompt in future sessions.", false)
}

func (s *Session) handleLoadSkill(tc backend.ToolCall) {
	var a struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Name) == "" {
		s.recordToolResult(tc, "error: load_skill requires a non-empty \"name\" string", true)
		return
	}
	content, ok := skills.Load(a.Name)
	if !ok {
		cat := skills.Catalog()
		names := make([]string, len(cat))
		for i, e := range cat {
			names[i] = e.Name
		}
		s.recordToolResult(tc, fmt.Sprintf("error: no skill named %q. Available: %s", a.Name, strings.Join(names, ", ")), true)
		return
	}
	s.recordToolResult(tc, content, false)
}

func (s *Session) handleViewImage(tc backend.ToolCall) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Path) == "" {
		s.recordToolResult(tc, "error: view_image requires a non-empty \"path\" string", true)
		return
	}
	b64, err := tools.ReadImageBase64(a.Path)
	if err != nil {
		s.recordToolResult(tc, "error: "+err.Error(), true)
		return
	}
	s.recordToolResultImage(tc, "image attached: "+a.Path, []string{b64})
}

// recordToolResultImage is recordToolResult but attaches image data to
// the tool-result message (view_image) instead of plain text.
func (s *Session) recordToolResultImage(tc backend.ToolCall, content string, images []string) {
	s.mu.Lock()
	s.lastFailedCall = ""
	s.repeatFailCount = 0
	s.messages = append(s.messages, backend.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Function.Name,
		Content:    content,
		Images:     images,
	})
	s.blocks = append(s.blocks, Block{Kind: BlockToolResult, ToolName: tc.Function.Name, Text: content})
	s.notify()
	s.mu.Unlock()
}

func (s *Session) recordToolResult(tc backend.ToolCall, content string, isError bool) {
	s.mu.Lock()
	callKey := tc.Function.Name + "\x00" + tc.Function.Arguments
	switch {
	case !isError:
		s.lastFailedCall = ""
		s.repeatFailCount = 0
	case callKey == s.lastFailedCall:
		s.repeatFailCount++
	default:
		s.lastFailedCall = callKey
		s.repeatFailCount = 1
	}

	s.messages = append(s.messages, backend.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Function.Name,
		Content:    content,
	})
	s.blocks = append(s.blocks, Block{Kind: BlockToolResult, ToolName: tc.Function.Name, Text: content, IsError: isError})
	s.notify()
	s.mu.Unlock()
}

func extractPath(argsJSON string) string {
	var a struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return a.Path
}

func recordUsage(tag string, usage backend.Usage, wallClock time.Duration) {
	genDur := wallClock
	if usage.EvalDurationNs > 0 {
		genDur = time.Duration(usage.EvalDurationNs)
	}
	_ = usagelog.Append(usagelog.Record{
		Time:             time.Now(),
		Model:            tag,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		DurationMs:       genDur.Milliseconds(),
	})
}

// notifyOnly locks just to call notify() when no other field changed.
func (s *Session) notifyOnly() {
	s.mu.Lock()
	s.notify()
	s.mu.Unlock()
}
