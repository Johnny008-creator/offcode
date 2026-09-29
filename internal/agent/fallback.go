package agent

import (
	"encoding/json"
	"strings"
)

// extractPseudoToolCall handles models that ignore the tools API and instead
// write a tool-call-shaped JSON object directly into the message content
// (observed with qwen2.5-coder:7b over Ollama, both /v1 and native /api/chat).
// It only fires when the whole trimmed content is a single JSON object, to
// avoid misfiring on prose that happens to mention JSON.
func extractPseudoToolCall(content string) (name string, argsJSON string, ok bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return "", "", false
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &generic); err != nil {
		return "", "", false
	}

	nameKey := firstPresentKey(generic, "name", "tool", "function")
	argsKey := firstPresentKey(generic, "arguments", "parameters", "args")
	if nameKey == "" || argsKey == "" {
		return "", "", false
	}

	if err := json.Unmarshal(generic[nameKey], &name); err != nil || name == "" {
		return "", "", false
	}

	return name, string(generic[argsKey]), true
}

func firstPresentKey(m map[string]json.RawMessage, candidates ...string) string {
	for _, c := range candidates {
		if _, ok := m[c]; ok {
			return c
		}
	}
	return ""
}
