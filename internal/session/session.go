// Package session persists a chat session's conversation per working
// directory, so running offcode again in the same project resumes where
// you left off instead of starting from zero.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type Saved struct {
	Model    string            `json:"model"`
	Messages []backend.Message `json:"messages"`
	SavedAt  time.Time         `json:"saved_at"`
}

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".offcode", "sessions")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}

// pathFor maps a working directory to a stable session file name. Hashed
// rather than sanitized-and-used-directly so arbitrarily deep/weird paths
// (unicode, excess length) never collide with filesystem limits.
func pathFor(cwd string) (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(cwd))
	return filepath.Join(d, hex.EncodeToString(sum[:16])+".json"), nil
}

// Load returns the saved session for cwd, and ok=false if there isn't one
// (not an error — the common case for a first run in a directory).
func Load(cwd string) (Saved, bool, error) {
	path, err := pathFor(cwd)
	if err != nil {
		return Saved{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Saved{}, false, nil
		}
		return Saved{}, false, err
	}
	var s Saved
	if err := json.Unmarshal(data, &s); err != nil {
		return Saved{}, false, err
	}
	return s, true, nil
}

// Save overwrites the saved session for cwd. Best-effort by design —
// callers should treat a save failure as non-fatal (same stance as
// usagelog.Append: persistence is a convenience, not something that
// should interrupt the chat loop).
func Save(cwd string, s Saved) error {
	path, err := pathFor(cwd)
	if err != nil {
		return err
	}
	s.SavedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Clear removes the saved session for cwd (used by -fresh to also wipe
// any stale save rather than just ignoring it, so a subsequent run
// without -fresh doesn't resurrect it).
func Clear(cwd string) error {
	path, err := pathFor(cwd)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
