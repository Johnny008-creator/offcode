// Package subagent runs a bounded, headless agent loop for delegating a
// focused side-task — a research question, "figure out why X," a test
// to write — to a model (possibly a different tier) and getting back a
// text summary, rather than the calling model doing it all inline. The
// same pattern Claude Code's own Agent tool uses; here it also lets a
// weaker tier punt a hard sub-problem to a stronger one.
//
// Deliberately read-only/investigative: a subagent runs with no human
// attached to approve anything, so it's given permission.ModePlan's
// tool set (read, grep, glob, web_search, fetch_url, browser_text) plus
// plan_set/plan_update for its own bookkeeping — never write/edit/bash.
// A subagent that needs to make changes should report back what it
// found and let the calling model (which DOES have a human attached,
// via the normal confirm flow) make them.
package subagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/tools"
)

const maxIterations = 15

const systemPromptTemplate = `You are a sub-agent spawned by another instance of offcode to investigate a
specific, bounded task and report back — you are not talking to the end user directly, your final
response goes back to the agent that spawned you, as the answer to its task.

Investigate thoroughly using your tools (read, grep, glob, web_search, fetch_url, browser_text,
plan_set, plan_update) — you do NOT have write/edit/bash, you cannot make changes, only investigate
and report. If the task implies a change should be made, describe exactly what should change and
where, so the calling agent can make it.

When you're done, give a complete, self-contained final answer with no tool calls — it's the only
thing the calling agent will see, so include everything it needs: your conclusion, the concrete
evidence/findings behind it, and specific file paths or line numbers where relevant. Be thorough but
not padded — every sentence should carry real information.

Task: %s`

func allowedTools() []backend.Tool {
	all := tools.Definitions()
	out := make([]backend.Tool, 0, len(all))
	for _, t := range all {
		if permission.IsReadOnly(t.Function.Name) {
			out = append(out, t)
		}
	}
	return out
}

// Result is what a subagent run reports back — Summary is the final
// answer; Trace is a short, ordered list of tool names it actually used,
// for the calling side to show as a one-line transparency note (e.g.
// "subagent used web_search, web_search, fetch_url across 3 steps").
type Result struct {
	Summary string
	Trace   []string
}

// Run drives the subagent loop to completion (a final text answer, or an
// error/iteration-limit). backendKind/modelTag/think select which model
// runs it, same as the calling agent's own request — a caller can pass a
// different, stronger tier to delegate up.
func Run(ctx context.Context, client *backend.Client, backendKind, modelTag string, think backend.ThinkLevel, task string) (Result, error) {
	messages := []backend.Message{
		{Role: "system", Content: fmt.Sprintf(systemPromptTemplate, task)},
	}
	toolDefs := allowedTools()
	var trace []string

	for i := 0; i < maxIterations; i++ {
		req := backend.ChatRequest{Model: modelTag, Messages: messages, Tools: toolDefs}
		msg, err := runOnce(ctx, client, backendKind, req, think)
		if err != nil {
			return Result{Trace: trace}, err
		}
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return Result{Summary: strings.TrimSpace(msg.Content), Trace: trace}, nil
		}

		for _, tc := range msg.ToolCalls {
			trace = append(trace, tc.Function.Name)
			out, err := dispatchRestricted(tc)
			if err != nil {
				out = "error: " + err.Error()
			}
			messages = append(messages, backend.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    out,
			})
		}
	}

	return Result{Trace: trace}, fmt.Errorf("subagent exceeded %d iterations without a final answer", maxIterations)
}

// dispatchRestricted runs a tool call under the same read-only
// restriction as allowedTools() — defense in depth, in case a model
// hallucinates a call to a tool it was never given (plan_set/plan_update
// are handled as harmless no-ops, since a subagent's internal plan isn't
// shown to anyone and isn't worth real bookkeeping for a single bounded
// task).
func dispatchRestricted(tc backend.ToolCall) (string, error) {
	switch tc.Function.Name {
	case "plan_set":
		return "plan noted (not tracked for sub-agents — just proceed with your investigation).", nil
	case "plan_update":
		return "ok", nil
	}
	if !permission.IsReadOnly(tc.Function.Name) {
		return "", fmt.Errorf("DENIED: sub-agents are read-only and cannot call %q", tc.Function.Name)
	}
	return tools.Dispatch(tc.Function.Name, tc.Function.Arguments)
}

// runOnce drains a single (possibly streamed) chat completion into its
// final message — a subagent has no UI to stream tokens into, so there's
// no reason to surface intermediate chunks.
func runOnce(ctx context.Context, client *backend.Client, backendKind string, req backend.ChatRequest, think backend.ThinkLevel) (backend.Message, error) {
	var ch <-chan backend.StreamEvent
	if backendKind == "ollama" {
		ch = client.StreamChatNative(ctx, req, think)
	} else {
		ch = client.StreamChatCompletion(ctx, req)
	}
	for ev := range ch {
		switch ev.Kind {
		case backend.EventError:
			return backend.Message{}, ev.Err
		case backend.EventDone:
			return ev.Message, nil
		}
	}
	return backend.Message{}, fmt.Errorf("stream closed unexpectedly")
}
