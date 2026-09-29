package tui

import (
	"fmt"
	"strings"
)

type blockKind int

const (
	blockUser blockKind = iota
	blockAssistant
	blockReasoning
	blockToolCall
	blockToolResult
	blockError
	blockSystem
)

type block struct {
	kind        blockKind
	text        string
	toolName    string
	toolArgs    string
	toolIsError bool
}

const maxResultLines = 12

func renderBlock(b block) string {
	switch b.kind {
	case blockUser:
		return userLabelStyle.Render("you") + "\n" + b.text
	case blockAssistant:
		if b.text == "" {
			return ""
		}
		return assistantLabelStyle.Render("offcode") + "\n" + b.text
	case blockReasoning:
		if strings.TrimSpace(b.text) == "" {
			return ""
		}
		return thinkingStyle.Render("thinking: " + strings.TrimSpace(b.text))
	case blockToolCall:
		header := toolLabelStyle.Render("→ " + b.toolName)
		args := toolArgsStyle.Render(truncate(b.toolArgs, 200))
		return header + "  " + args
	case blockToolResult:
		label := "result"
		style := resultLabelStyle
		if b.toolIsError {
			label = "error"
			style = errorStyle
		}
		body := clampLines(b.text, maxResultLines)
		return style.Render(label) + "\n" + resultBodyStyle.Render(body)
	case blockError:
		return errorStyle.Render("error: " + b.text)
	case blockSystem:
		return systemStyle.Render(b.text)
	}
	return b.text
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func clampLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	shown := lines[:n]
	return strings.Join(shown, "\n") + fmt.Sprintf("\n… (%d more lines)", len(lines)-n)
}
