package tools

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

func Bash(command string) (string, error) {
	return BashContext(context.Background(), command)
}

func BashContext(parent context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
