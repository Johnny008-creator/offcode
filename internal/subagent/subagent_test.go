package subagent

import (
	"testing"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func TestDispatchRestrictedDeniesWrite(t *testing.T) {
	_, err := dispatchRestricted(toolCall("write", `{"path":"x.txt","content":"y"}`))
	if err == nil {
		t.Fatal("expected write to be denied for a sub-agent")
	}
}

func TestDispatchRestrictedDeniesBash(t *testing.T) {
	_, err := dispatchRestricted(toolCall("bash", `{"command":"ls"}`))
	if err == nil {
		t.Fatal("expected bash to be denied for a sub-agent")
	}
}

func TestDispatchRestrictedAllowsGrep(t *testing.T) {
	// grep over the package's own directory — exercising the real
	// allowed path, not just that it isn't denied.
	_, err := dispatchRestricted(toolCall("grep", `{"pattern":"package subagent","path":"."}`))
	if err != nil {
		t.Fatalf("expected grep to be allowed for a sub-agent, got: %v", err)
	}
}

func TestDispatchRestrictedPlanToolsAreNoops(t *testing.T) {
	if _, err := dispatchRestricted(toolCall("plan_set", `{"steps":["a"]}`)); err != nil {
		t.Errorf("plan_set should be a harmless no-op, got error: %v", err)
	}
	if _, err := dispatchRestricted(toolCall("plan_update", `{"index":0,"status":"done"}`)); err != nil {
		t.Errorf("plan_update should be a harmless no-op, got error: %v", err)
	}
}

func TestAllowedToolsContainsOnlyReadOnly(t *testing.T) {
	for _, tl := range allowedTools() {
		switch tl.Function.Name {
		case "write", "edit", "bash":
			t.Errorf("allowedTools() must never include %q", tl.Function.Name)
		}
	}
	if len(allowedTools()) == 0 {
		t.Error("expected at least some read-only tools (read/grep/glob/web_search/fetch_url/browser_text)")
	}
}

// toolCall is a small test helper building a backend.ToolCall inline.
func toolCall(name, argsJSON string) backend.ToolCall {
	return backend.ToolCall{
		ID:       "test",
		Type:     "function",
		Function: backend.FunctionCall{Name: name, Arguments: argsJSON},
	}
}
