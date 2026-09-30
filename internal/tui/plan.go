package tui

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func metaSchema(props string, required ...string) json.RawMessage {
	req, _ := json.Marshal(required)
	return json.RawMessage(fmt.Sprintf(`{"type":"object","properties":%s,"required":%s}`, props, req))
}

func metaToolDefinitions() []backend.Tool {
	return []backend.Tool{
		{Type: "function", Function: backend.FunctionDef{
			Name: "plan_set",
			Description: "Set or replace your step-by-step plan for the current task. Call this first " +
				"for any non-trivial multi-step task, before taking any action. Each step should be a short, concrete description.",
			Parameters: metaSchema(`{"steps":{"type":"array","items":{"type":"string"},"description":"Ordered list of step descriptions"}}`, "steps"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name: "plan_update",
			Description: "Update the status of one step in your current plan. Call this as you start " +
				"and finish each step, so progress stays visible to the user.",
			Parameters: metaSchema(`{"index":{"type":"integer","description":"0-based index of the step"},"status":{"type":"string","enum":["pending","in_progress","done","skipped"]}}`, "index", "status"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name: "ask_user",
			Description: "Ask the user to choose between multiple reasonable options when the right approach " +
				"is genuinely ambiguous and you have a recommendation. Don't use this for things you can reasonably decide yourself.",
			Parameters: metaSchema(`{"question":{"type":"string"},"options":{"type":"array","items":{"type":"string"},"description":"At least 2 options"},"recommended_index":{"type":"integer","description":"0-based index of the option you recommend, if any"}}`, "question", "options"),
		}},
	}
}

type planStepStatus int

const (
	stepPending planStepStatus = iota
	stepInProgress
	stepDone
	stepSkipped
)

type planStep struct {
	text   string
	status planStepStatus
}

func parsePlanStatus(s string) (planStepStatus, bool) {
	switch s {
	case "pending":
		return stepPending, true
	case "in_progress":
		return stepInProgress, true
	case "done":
		return stepDone, true
	case "skipped":
		return stepSkipped, true
	}
	return stepPending, false
}

func (m Model) handlePlanSet(tc backend.ToolCall) (tea.Model, tea.Cmd) {
	var a struct {
		Steps []string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || len(a.Steps) == 0 {
		return m.recordToolResult(tc, "error: plan_set requires a non-empty \"steps\" array of strings", true)
	}

	m.planSteps = make([]planStep, len(a.Steps))
	for i, s := range a.Steps {
		m.planSteps[i] = planStep{text: s}
	}
	m.resizeViewport()
	return m.recordToolResult(tc, fmt.Sprintf("plan set with %d steps", len(a.Steps)), false)
}

func (m Model) handlePlanUpdate(tc backend.ToolCall) (tea.Model, tea.Cmd) {
	var a struct {
		Index  int    `json:"index"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil {
		return m.recordToolResult(tc, "error: invalid plan_update arguments", true)
	}
	if a.Index < 0 || a.Index >= len(m.planSteps) {
		return m.recordToolResult(tc, fmt.Sprintf("error: step index %d out of range (plan has %d steps, 0-indexed)", a.Index, len(m.planSteps)), true)
	}
	status, ok := parsePlanStatus(a.Status)
	if !ok {
		return m.recordToolResult(tc, "error: status must be one of: pending, in_progress, done, skipped", true)
	}

	m.planSteps[a.Index].status = status
	return m.recordToolResult(tc, fmt.Sprintf("step %d marked %s", a.Index, a.Status), false)
}

func (m Model) handleAskUser(tc backend.ToolCall) (tea.Model, tea.Cmd) {
	var a struct {
		Question         string   `json:"question"`
		Options          []string `json:"options"`
		RecommendedIndex *int     `json:"recommended_index"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || a.Question == "" || len(a.Options) < 2 {
		return m.recordToolResult(tc, "error: ask_user requires a question and at least 2 options", true)
	}

	items := make([]string, len(a.Options))
	cursor := 0
	for i, opt := range a.Options {
		items[i] = opt
		if a.RecommendedIndex != nil && *a.RecommendedIndex == i {
			items[i] += "  (recommended)"
			cursor = i
		}
	}

	m.picker = pickerAskUser
	m.pickerReturnTo = pickerNone
	m.pickerItems = items
	m.pickerCursor = cursor
	m.pickerQuestion = a.Question
	m.pickerAskOptions = a.Options
	tcCopy := tc
	m.pendingAskUser = &tcCopy
	m.activity = "waiting for your answer"
	m.resizeViewport()
	return m, nil
}

func (m Model) resolveAskUser() (tea.Model, tea.Cmd) {
	if m.pendingAskUser == nil {
		m.picker = pickerNone
		return m, nil
	}
	tc := *m.pendingAskUser
	m.pendingAskUser = nil

	answer := ""
	if m.pickerCursor < len(m.pickerAskOptions) {
		answer = m.pickerAskOptions[m.pickerCursor]
	}

	m.picker = pickerNone
	m.resizeViewport()
	return m.recordToolResult(tc, answer, false)
}

func (m Model) cancelAskUser() (tea.Model, tea.Cmd) {
	if m.pendingAskUser == nil {
		m.picker = pickerNone
		return m, nil
	}
	tc := *m.pendingAskUser
	m.pendingAskUser = nil

	m.picker = pickerNone
	m.resizeViewport()
	return m.recordToolResult(tc, "user dismissed the question without answering", true)
}
