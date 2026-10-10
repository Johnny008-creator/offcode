//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachOllamaProcess puts the spawned `ollama serve` in its own session,
// so a Ctrl+C that sends SIGINT to offcode's foreground process group
// (normal terminal behavior) doesn't also kill the backend offcode just
// started on the user's behalf.
func detachOllamaProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
