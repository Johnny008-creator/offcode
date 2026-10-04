package tui

import "time"

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
