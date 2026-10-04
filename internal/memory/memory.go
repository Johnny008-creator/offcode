// Package memory persists free-form notes across sessions, independent of
// any one working directory's session.json (internal/session). It's the
// one thing the model — or the user — can deliberately choose to carry
// forward, like "this project uses pnpm, not npm" or "user prefers tabs."
//
// Deliberately a single flat file rather than a typed/tagged store: a
// smaller model's ability to notice when something is memory-worthy is
// unreliable, so the primary path is expected to be the explicit
// /remember command, with the remember tool as a bonus when a model does
// use it well. Simpler format means nothing to get wrong on either path.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".offcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory.md"), nil
}

// Append adds a timestamped note to ~/.offcode/memory.md. Best-effort by
// design, same stance as usagelog/session: persistence is a convenience,
// never something that should interrupt the chat loop.
func Append(note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	p, err := path()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- [%s] %s\n", time.Now().Format("2006-01-02"), note)
	return err
}

// ReadAll returns the current memory file content, or "" if none exists
// yet (not an error — the common case before anything is ever remembered).
func ReadAll() (string, error) {
	p, err := path()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}
