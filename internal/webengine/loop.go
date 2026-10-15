package webengine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/agent"
	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/hooks"
	"github.com/Johnny008-creator/offcode/internal/mcpclient"
	"github.com/Johnny008-creator/offcode/internal/memory"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/semsearch"
	"github.com/Johnny008-creator/offcode/internal/skills"
	"github.com/Johnny008-creator/offcode/internal/subagent"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/tools"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
)

// runTurn drives the agent loop to completion (a final answer, an error,
// or a cancellation), pausing synchronously on tool confirmation and
// ask_user via blocking channel receives. It always leaves the session in
// status "idle" when it returns, and always runs in its own goroutine
// (started by Submit/SubmitDisplay).
func (s *Session) runTurn(ctx context.Context) {
	for {
		msg, usage, turnStart, err := s.streamOnceWithParseRetry(ctx)
		if err != nil {
			s.finishRun(err)
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
			incomplete := s.incompletePlanSteps()
			if len(incomplete) > 0 && s.planNudges < maxPlanNudges {
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
			// Live-observed with gemma4:12b/Anvil: after the nudge budget runs
			// out, the model can still reply with empty content and no tool
			// calls — finishRun(nil) would then end the turn with literally
			// nothing visible, leaving the user staring at a conversation
			// that just stopped with no explanation (the plan panel shows
			// unfinished steps, but nothing says why generation ended). Make
			// that outcome visible instead of silent.
			if len(incomplete) > 0 && msg.Content == "" {
				s.pushBlock(Block{Kind: BlockSystem, Text: fmt.Sprintf(
					"( stopped without a final answer — %d plan step(s) still unfinished after %d reminder(s): %s )",
					len(incomplete), maxPlanNudges, strings.Join(incomplete, "; "),
				)})
			}
			// Live-observed with qwen3:1.7b and no plan: after a round of
			// failed tool calls the model replied with nothing at all, and
			// the turn ended silently. Give it one chance to recover or
			// answer, then say visibly that it stopped.
			if len(incomplete) == 0 && strings.TrimSpace(msg.Content) == "" {
				s.mu.Lock()
				nudged := s.emptyNudged
				s.emptyNudged = true
				s.mu.Unlock()
				if !nudged {
					s.pushBlock(Block{Kind: BlockSystem, Text: "( empty reply — asking the model to continue or answer )"})
					s.mu.Lock()
					s.messages = append(s.messages, backend.Message{Role: "user", Content: emptyReplyNudge})
					s.mu.Unlock()
					continue
				}
				s.pushBlock(Block{Kind: BlockSystem, Text: "( stopped without a final answer — the model replied with nothing after a reminder )"})
			}
			s.finishRun(nil)
			return
		}

		s.toolIterations++
		if s.toolIterations > maxToolIterationsPerTurn {
			s.finishRun(fmt.Errorf("stopped after %d tool iterations without a final answer", maxToolIterationsPerTurn))
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

// maxToolCallParseRetries bounds how many extra attempts streamOnceWithParseRetry
// makes after Ollama fails to parse a model's own tool-call output (see
// backend.IsMalformedToolCallResponse) — a different sampling run of the
// same prompt has a real chance of producing well-formed tags.
const maxToolCallParseRetries = 2

// streamOnceWithParseRetry wraps streamOnce, retrying up to
// maxToolCallParseRetries times when the only thing that failed was
// Ollama's own tool-call template parsing — any other error (backend
// unreachable, model missing, context canceled) passes through on the
// first attempt, unchanged.
func (s *Session) streamOnceWithParseRetry(ctx context.Context) (backend.Message, backend.Usage, time.Time, error) {
	for attempt := 0; ; attempt++ {
		msg, usage, turnStart, err := s.streamOnce(ctx)
		if err == nil || attempt >= maxToolCallParseRetries || !backend.IsMalformedToolCallResponse(err) {
			return msg, usage, turnStart, err
		}
	}
}

// streamOnce runs exactly one model request to completion (or error),
// streaming content/reasoning deltas into the transcript live.
func (s *Session) streamOnce(ctx context.Context) (backend.Message, backend.Usage, time.Time, error) {
	s.mu.Lock()
	model := s.cfg.Model
	backendKind := s.cfg.Backend
	think := s.think
	vision := s.visionCapable
	mcpTools := s.mcpTools
	msgs := append([]backend.Message(nil), s.messages...)
	s.activity = "thinking"
	s.mu.Unlock()

	req := backend.ChatRequest{Model: model, Messages: msgs, Tools: toolsForModel(model, vision, mcpTools)}

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
	if err := s.toolContext().Err(); err != nil {
		s.recordToolResult(tc, "interrupted: "+err.Error(), true)
		return
	}
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
	case "preview_page":
		s.handlePreviewPage(tc)
		return
	case "semantic_search":
		s.handleSemanticSearch(tc)
		return
	case "spawn_subagent":
		s.handleSpawnSubagent(tc)
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
	hooksCfg := s.hooksCfg
	s.mu.Unlock()

	if hookRes := hooks.RunPre(hooksCfg, tc.Function.Name, tc.Function.Arguments); hookRes.Denied {
		s.recordToolResult(tc, "DENIED by pre_tool_use hook: "+hookRes.Reason, true)
		return
	}

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

	var out string
	var err error
	if mcpclient.IsMCPTool(tc.Function.Name) {
		s.mu.Lock()
		mgr := s.mcpMgr
		s.mu.Unlock()
		mcpCtx, mcpCancel := context.WithTimeout(s.toolContext(), 60*time.Second)
		out, err = mgr.Dispatch(mcpCtx, tc.Function.Name, tc.Function.Arguments)
		mcpCancel()
	} else {
		out, err = tools.DispatchContext(s.toolContext(), tc.Function.Name, tc.Function.Arguments)
	}
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
	for _, warning := range hooks.RunPost(hooksCfg, tc.Function.Name, tc.Function.Arguments, out) {
		s.pushBlock(Block{Kind: BlockSystem, Text: warning})
	}
	s.recordToolResult(tc, out, false)
}

func (s *Session) waitForConfirm(tc backend.ToolCall) bool {
	ch := make(chan bool, 1)
	s.mu.Lock()
	s.pendingConfirmTC = &tc
	s.requestSequence++
	s.pendingConfirmID = fmt.Sprintf("%s:%d", s.sessionID, s.requestSequence)
	s.pendingPreview = diffPreview(tc)
	s.confirmCh = ch
	s.status = "confirming"
	s.activity = "awaiting confirmation"
	s.notify()
	s.mu.Unlock()

	var allow bool
	select {
	case allow = <-ch:
	case <-s.toolContext().Done():
	}

	s.mu.Lock()
	s.pendingConfirmTC = nil
	s.pendingConfirmID = ""
	s.confirmCh = nil
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
	s.requestSequence++
	s.pendingAskID = fmt.Sprintf("%s:%d", s.sessionID, s.requestSequence)
	s.pendingAskQuestion = a.Question
	s.pendingAskOptions = a.Options
	s.pendingAskRecommended = rec
	s.askCh = ch
	s.status = "asking"
	s.activity = "waiting for your answer"
	s.notify()
	s.mu.Unlock()

	var answer string
	select {
	case answer = <-ch:
	case <-s.toolContext().Done():
		answer = "interrupted"
	}

	s.mu.Lock()
	s.pendingAskTC = nil
	s.pendingAskID = ""
	s.askCh = nil
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
		// Seen live with qwen3:8b: it did every step's work but only marked the
		// last one done. Auto-promoting the next pending step then left several
		// steps in_progress at once. Point at the skipped-over steps instead.
		var open []string
		inProgress := false
		for i := range s.plan {
			finished := s.plan[i].Status == stepDone || s.plan[i].Status == stepSkipped
			if i < a.Index && !finished {
				open = append(open, strconv.Itoa(i))
			}
			inProgress = inProgress || s.plan[i].Status == stepInProgress
		}
		nextIdx := -1
		for i := range s.plan {
			if s.plan[i].Status == stepPending {
				nextIdx = i
				break
			}
		}
		switch {
		case len(open) > 0:
			result += fmt.Sprintf(" Earlier step(s) %s are not marked done yet — if you already did that work,"+
				" mark each one done now; otherwise do it before finishing.", strings.Join(open, ", "))
		case inProgress:
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

func (s *Session) handlePreviewPage(tc backend.ToolCall) {
	var a struct {
		Path   string `json:"path"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Path) == "" {
		s.recordToolResult(tc, "error: preview_page requires a non-empty \"path\" string", true)
		return
	}
	png, err := tools.PreviewPage(a.Path, a.Width, a.Height)
	if err != nil {
		s.recordToolResult(tc, "error: "+err.Error(), true)
		return
	}
	b64 := base64.StdEncoding.EncodeToString(png)
	s.recordToolResultImage(tc, "screenshot captured: "+a.Path, []string{b64})
}

func (s *Session) handleSpawnSubagent(tc backend.ToolCall) {
	var a struct {
		Task string `json:"task"`
		Tier string `json:"tier"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Task) == "" {
		s.recordToolResult(tc, "error: spawn_subagent requires a non-empty \"task\" string", true)
		return
	}

	s.mu.Lock()
	modelTag := s.cfg.Model
	backendKind := s.cfg.Backend
	think := s.think
	s.activity = "subagent working: " + a.Task
	s.notify()
	s.mu.Unlock()

	if a.Tier != "" {
		if t, ok := tiers.ByShortName(a.Tier); ok {
			modelTag = t.Tag
		} else {
			s.recordToolResult(tc, fmt.Sprintf("error: unknown tier %q (expected spark, ember, forge, or anvil)", a.Tier), true)
			return
		}
	}

	ctx, cancel := context.WithTimeout(s.toolContext(), 5*time.Minute)
	defer cancel()
	result, err := subagent.Run(ctx, s.client, backendKind, modelTag, think, a.Task)
	if err != nil {
		s.recordToolResult(tc, "error: subagent failed: "+err.Error(), true)
		return
	}
	var trace string
	if len(result.Trace) > 0 {
		trace = fmt.Sprintf(" (used: %s)", strings.Join(result.Trace, ", "))
	}
	s.recordToolResult(tc, fmt.Sprintf("subagent report%s:\n\n%s", trace, result.Summary), false)
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
	s.blocks = append(s.blocks, Block{Kind: BlockToolResult, ToolName: tc.Function.Name, Text: content, Images: images})
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
	b := artifactBlock(tc, content, isError, len(s.blocks))
	if b.ArtifactURL != "" {
		b.ArtifactURL += "&session=" + s.sessionID
	}
	s.blocks = append(s.blocks, b)
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

func (s *Session) handleSemanticSearch(tc backend.ToolCall) {
	var a struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || strings.TrimSpace(a.Query) == "" {
		s.recordToolResult(tc, "error: semantic_search requires a non-empty query string", true)
		return
	}
	if s.cfg.Backend != "ollama" {
		s.recordToolResult(tc, "error: semantic_search requires an Ollama backend with nomic-embed-text installed", true)
		return
	}
	if a.MaxResults <= 0 {
		a.MaxResults = 8
	}
	a.MaxResults = min(a.MaxResults, 50)
	ctx, cancel := context.WithTimeout(s.toolContext(), 3*time.Minute)
	defer cancel()
	matches, n, err := semsearch.Search(ctx, s.client, s.cwd, a.Query, a.MaxResults)
	if err != nil {
		s.recordToolResult(tc, "error: semantic_search failed: "+err.Error(), true)
		return
	}
	var b strings.Builder
	if n > 0 {
		fmt.Fprintf(&b, "(indexed %d file(s) for this search)\n\n", n)
	}
	if len(matches) == 0 {
		b.WriteString("no indexed content — the project may have no matching text files.")
	}
	for i, m := range matches {
		fmt.Fprintf(&b, "%d. %s:%d-%d (score %.2f)\n%s\n\n", i+1, m.File, m.StartLine, m.EndLine, m.Score, string([]rune(m.Text)[:min(len([]rune(m.Text)), 500)]))
	}
	s.recordToolResult(tc, strings.TrimSpace(b.String()), false)
}

func (s *Session) toolContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turnCtx != nil {
		return s.turnCtx
	}
	return context.Background()
}

// Publish idle only once final blocks and the saved session are consistent.
func (s *Session) finishRun(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if errors.Is(err, context.Canceled) {
		s.blocks = append(s.blocks, Block{Kind: BlockSystem, Text: "( interrupted )"})
	} else if err != nil {
		s.blocks = append(s.blocks, Block{Kind: BlockError, Text: err.Error(), IsError: true})
	}
	s.activity = ""
	s.saveToDiskLocked()
	s.status = "idle"
	s.notify()
}
