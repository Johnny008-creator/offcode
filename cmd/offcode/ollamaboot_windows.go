//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detachOllamaProcess puts the spawned `ollama serve` in its own process
// group, so a console Ctrl+C aimed at offcode doesn't also kill the
// backend offcode just started on the user's behalf.
func detachOllamaProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
