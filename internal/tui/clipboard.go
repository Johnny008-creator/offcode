package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"
)

type clipboardCopiedMsg struct {
	chars int
	err   error
}

// transcriptPlainText renders every visible block (respecting the
// showThinking filter, same as the viewport) as plain text with ANSI
// styling stripped, for copying to the system clipboard.
func (m Model) transcriptPlainText() string {
	var parts []string
	for _, b := range m.blocks {
		if b.kind == blockReasoning && !m.showThinking {
			continue
		}
		r := renderBlock(b)
		if r != "" {
			parts = append(parts, ansi.Strip(r))
		}
	}
	return strings.Join(parts, "\n\n")
}

// copyToClipboardCmd writes an OSC 52 clipboard-set escape sequence
// directly to stdout. This works even over SSH and doesn't require any
// terminal-native text selection, which alt-screen repaints (e.g. from
// scrolling) otherwise clear.
func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return clipboardCopiedMsg{}
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(text))
		seq := fmt.Sprintf("\x1b]52;c;%s\a", encoded)
		if _, err := os.Stdout.WriteString(seq); err != nil {
			return clipboardCopiedMsg{err: err}
		}
		return clipboardCopiedMsg{chars: len(text)}
	}
}
