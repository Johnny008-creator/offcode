package tui

import (
	"fmt"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

// Cheap-version context compaction, as scoped in docs/ROADMAP.md: no extra
// LLM call to summarize (that needs real state-machine work to avoid
// racing with the user's next input), just a mechanical extraction of
// what was asked and what tools touched what, replacing the oldest
// messages once the history gets long. Local models run with much
// smaller effective context than Claude (8-32K vs 200K), so this matters
// more here than it would for Claude Code itself.
const (
	compactionTriggerMessages = 40
	compactionKeepRecent      = 16
)

func (m *Model) maybeCompact() {
	if len(m.messages) <= compactionTriggerMessages {
		return
	}
	// messages[0] is the system prompt — never compacted away.
	sys := m.messages[0]
	cut := len(m.messages) - compactionKeepRecent
	if cut <= 1 {
		return
	}

	old := m.messages[1:cut]
	recent := m.messages[cut:]
	summary := summarizeMechanically(old)

	newMessages := make([]backend.Message, 0, 2+len(recent))
	newMessages = append(newMessages, sys, backend.Message{Role: "user", Content: summary})
	newMessages = append(newMessages, recent...)
	m.messages = newMessages

	m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf(
		"context compacted: summarized %d older messages to stay within context (kept the system prompt and the last %d messages in full)",
		len(old), compactionKeepRecent,
	)})
}

func summarizeMechanically(msgs []backend.Message) string {
	var asks []string
	var tools []string
	filesTouched := map[string]bool{}

	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			if msg.Content != "" {
				asks = append(asks, truncate(msg.Content, 100))
			}
		case "assistant":
			for _, tc := range msg.ToolCalls {
				tools = append(tools, tc.Function.Name)
				if path := extractPath(tc.Function.Arguments); path != "" {
					filesTouched[path] = true
				}
			}
		}
	}

	var b strings.Builder
	b.WriteString("[Earlier conversation summarized to save context — ")
	fmt.Fprintf(&b, "%d messages omitted.]\n", len(msgs))
	if len(asks) > 0 {
		b.WriteString("Requests made: " + strings.Join(asks, " | ") + "\n")
	}
	if len(tools) > 0 {
		b.WriteString("Tools used: " + strings.Join(tools, ", ") + "\n")
	}
	if len(filesTouched) > 0 {
		files := make([]string, 0, len(filesTouched))
		for f := range filesTouched {
			files = append(files, f)
		}
		b.WriteString("Files touched: " + strings.Join(files, ", ") + "\n")
	}
	b.WriteString("Continue from here using the recent messages below for exact detail.")
	return b.String()
}
