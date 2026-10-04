// Package session persists chat conversations per working directory, so
// running offcode again in the same project can resume a past
// conversation instead of starting from zero. A directory can hold
// multiple sessions (e.g. one per distinct task) — nothing auto-resumes;
// callers decide when to list/load, matching the --resume opt-in model.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type Saved struct {
	Model     string            `json:"model"`
	Messages  []backend.Message `json:"messages"`
	StartedAt time.Time         `json:"started_at"`
	SavedAt   time.Time         `json:"saved_at"`
}

// Meta is a saved session's listing info, without loading its full
// message history — cheap enough to build for every session in a
// directory just to render a picker/sidebar.
type Meta struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	SavedAt   time.Time `json:"saved_at"`
	Messages  int       `json:"messages"`
	Summary   string    `json:"summary"`
}

// dirFor returns (creating if needed) the directory holding every saved
// session for cwd. Hashed rather than sanitized-and-used-directly so
// arbitrarily deep/weird paths (unicode, excess length) never collide
// with filesystem limits.
func dirFor(cwd string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(cwd))
	d := filepath.Join(home, ".offcode", "sessions", hex.EncodeToString(sum[:16]))
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}

func pathFor(cwd, id string) (string, error) {
	d, err := dirFor(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, id+".json"), nil
}

// NewID mints a fresh session id. Timestamp-based (to millisecond
// precision) so sessions sort chronologically by filename alone and
// collisions require two sessions created in the same process within
// the same millisecond, which doesn't happen in practice.
func NewID() string {
	return time.Now().Format("20060102-150405.000")
}

// List returns every saved session for cwd, newest first. Returns an
// empty (non-nil) slice and no error if cwd has never had a session —
// not a failure case, the common one for a brand-new project.
func List(cwd string) ([]Meta, error) {
	d, err := dirFor(cwd)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil, err
	}

	metas := make([]Meta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		saved, ok, err := Load(cwd, id)
		if err != nil || !ok {
			continue
		}
		metas = append(metas, Meta{
			ID:        id,
			StartedAt: saved.StartedAt,
			SavedAt:   saved.SavedAt,
			Messages:  len(saved.Messages),
			Summary:   summarize(saved.Messages),
		})
	}

	sort.Slice(metas, func(i, j int) bool { return metas[i].SavedAt.After(metas[j].SavedAt) })
	return metas, nil
}

// summarize picks the first user message as a short display label for a
// session picker/sidebar.
func summarize(msgs []backend.Message) string {
	for _, m := range msgs {
		if m.Role == "user" && m.Content != "" {
			s := strings.Join(strings.Fields(m.Content), " ")
			if len(s) > 70 {
				s = s[:70] + "…"
			}
			return s
		}
	}
	return "(empty session)"
}

// Load returns the saved session with the given id for cwd, and
// ok=false if it doesn't exist (not an error — a session id can be
// stale if its file was removed outside offcode).
func Load(cwd, id string) (Saved, bool, error) {
	path, err := pathFor(cwd, id)
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

// Save overwrites the session with the given id for cwd, creating it if
// new. Best-effort by design — callers should treat a save failure as
// non-fatal (same stance as usagelog.Append: persistence is a
// convenience, never something that should interrupt the chat loop).
func Save(cwd, id string, s Saved) error {
	path, err := pathFor(cwd, id)
	if err != nil {
		return err
	}
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now()
	}
	s.SavedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Delete removes one saved session for cwd.
func Delete(cwd, id string) error {
	path, err := pathFor(cwd, id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
