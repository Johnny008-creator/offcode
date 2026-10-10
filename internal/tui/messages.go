package tui

import (
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/mcpclient"
	"github.com/Johnny008-creator/offcode/internal/semsearch"
	"github.com/Johnny008-creator/offcode/internal/subagent"
)

type splashDoneMsg struct {
	contextLimit  int
	visionCapable bool
}

type toolExecutedMsg struct {
	callID string
	name   string
	args   string
	path   string
	output string
	err    error
}

type tickMsg time.Time

type modelsLoadedMsg struct {
	models []string
	err    error
}

type contextRefreshedMsg struct {
	limit         int
	visionCapable bool
}

type mcpConnectedMsg struct {
	mgr      *mcpclient.Manager
	tools    []backend.Tool
	warnings []string
}

type subagentDoneMsg struct {
	callID string
	result subagent.Result
	err    error
}

type semanticSearchDoneMsg struct {
	callID     string
	matches    []semsearch.Match
	reembedded int
	err        error
}
