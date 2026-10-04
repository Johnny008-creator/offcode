package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type diffLineKind int

const (
	diffContext diffLineKind = iota
	diffRemoved
	diffAdded
)

type diffLine struct {
	kind diffLineKind
	text string
}

// lineDiff trims the common prefix and suffix between oldText and newText
// and returns only the differing middle as removed/added lines, capped at
// maxLines. Good enough for a confirmation preview without needing a real
// diff algorithm — for typical edits (change a few lines in the middle of
// a file, or a short old_string/new_string pair) it shows exactly the
// change; for a full-file rewrite it just shows everything as
// removed+added, which is still far more useful than raw truncated JSON.
func lineDiff(oldText, newText string, maxLines int) []diffLine {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	removed := oldLines[prefix : len(oldLines)-suffix]
	added := newLines[prefix : len(newLines)-suffix]

	if len(removed) == 1 && removed[0] == "" {
		removed = nil
	}
	if len(added) == 1 && added[0] == "" {
		added = nil
	}
	if len(removed) == 0 && len(added) == 0 {
		return []diffLine{{diffContext, "(no textual change)"}}
	}

	var out []diffLine
	for _, l := range removed {
		out = append(out, diffLine{diffRemoved, l})
	}
	for _, l := range added {
		out = append(out, diffLine{diffAdded, l})
	}

	if len(out) > maxLines {
		more := len(out) - maxLines
		out = out[:maxLines]
		out = append(out, diffLine{diffContext, fmt.Sprintf("… (%d more diff lines)", more)})
	}
	return out
}

func previewLines(count int, lines []string) []diffLine {
	n := len(lines)
	if n > count {
		n = count
	}
	out := make([]diffLine, 0, n+1)
	for _, l := range lines[:n] {
		out = append(out, diffLine{diffAdded, l})
	}
	if len(lines) > count {
		out = append(out, diffLine{diffContext, fmt.Sprintf("… (%d more lines)", len(lines)-count)})
	}
	return out
}

// confirmPreview builds a human-reviewable preview of what a tool call
// will actually do, for the y/n confirmation prompt — a diff for
// write/edit instead of raw (and previously truncated) JSON arguments,
// which gave no real basis to decide.
func confirmPreview(tc backend.ToolCall) (header string, lines []diffLine) {
	switch tc.Function.Name {
	case "write":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || a.Path == "" {
			break
		}
		if old, err := os.ReadFile(a.Path); err == nil {
			return "overwriting " + a.Path, lineDiff(string(old), a.Content, 12)
		}
		return "new file " + a.Path, previewLines(10, strings.Split(a.Content, "\n"))

	case "edit":
		var a struct {
			Path      string `json:"path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &a); err != nil || a.Path == "" {
			break
		}
		return "editing " + a.Path, lineDiff(a.OldString, a.NewString, 12)
	}

	return tc.Function.Name, []diffLine{{diffContext, truncate(tc.Function.Arguments, 300)}}
}
