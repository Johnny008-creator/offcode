package tui

import "time"

type splashDoneMsg struct {
	contextLimit int
}

type toolExecutedMsg struct {
	callID string
	name   string
	output string
	err    error
}

type tickMsg time.Time
