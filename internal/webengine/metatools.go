package webengine

import (
	"encoding/json"
	"fmt"

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
		{Type: "function", Function: backend.FunctionDef{
			Name: "remember",
			Description: "Save a short, durable note that should carry over into future sessions in " +
				"this or any project — a user preference, a project convention, a fact worth not re-discovering. " +
				"Don't use this for anything specific to just the current task.",
			Parameters: metaSchema(`{"note":{"type":"string","description":"One short, self-contained sentence."}}`, "note"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name: "load_skill",
			Description: "Load the full reference content of a named skill from the available-skills list " +
				"in your system prompt, when the current task actually touches that topic.",
			Parameters: metaSchema(`{"name":{"type":"string","description":"Exact skill name from the available-skills list."}}`, "name"),
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

func (s planStepStatus) String() string {
	switch s {
	case stepInProgress:
		return "in_progress"
	case stepDone:
		return "done"
	case stepSkipped:
		return "skipped"
	default:
		return "pending"
	}
}

type planStep struct {
	Text   string
	Status planStepStatus
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
