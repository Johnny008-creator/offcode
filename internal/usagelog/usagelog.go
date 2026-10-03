// Package usagelog persists per-request token usage to a small JSONL file
// shared between the chat TUI and `offcode monitor`, so the monitor can
// show usage that accumulated across past sessions, not just whatever is
// currently running.
package usagelog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/tiers"
)

type Record struct {
	Time             time.Time `json:"time"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	DurationMs       int64     `json:"duration_ms"`
}

func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".offcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "usage.log"), nil
}

// Append adds one usage record. Best-effort: a failure here (e.g. a
// read-only home directory) should never interrupt the chat loop, so
// callers typically ignore the returned error or just log it quietly.
func Append(rec Record) error {
	path, err := logPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// ReadAll loads every recorded usage entry. Returns an empty slice (not an
// error) if the log doesn't exist yet.
func ReadAll() ([]Record, error) {
	path, err := logPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

type Summary struct {
	Tag              string
	Requests         int
	PromptTokens     int
	CompletionTokens int
	Duration         time.Duration
}

func (s Summary) TotalTokens() int { return s.PromptTokens + s.CompletionTokens }

func (s Summary) TokensPerSec() float64 {
	secs := s.Duration.Seconds()
	if secs <= 0 {
		return 0
	}
	return float64(s.CompletionTokens) / secs
}

// Summarize aggregates records per model, sorted by total tokens
// descending. Pass a since value to only include records at or after that
// time (zero value includes everything).
func Summarize(records []Record, since time.Time) []Summary {
	byTag := map[string]*Summary{}
	var order []string
	for _, rec := range records {
		if !since.IsZero() && rec.Time.Before(since) {
			continue
		}
		s, ok := byTag[rec.Model]
		if !ok {
			s = &Summary{Tag: rec.Model}
			byTag[rec.Model] = s
			order = append(order, rec.Model)
		}
		s.Requests++
		s.PromptTokens += rec.PromptTokens
		s.CompletionTokens += rec.CompletionTokens
		s.Duration += time.Duration(rec.DurationMs) * time.Millisecond
	}

	out := make([]Summary, 0, len(order))
	for _, tag := range order {
		out = append(out, *byTag[tag])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalTokens() > out[j].TotalTokens() })
	return out
}

const reportBarWidth = 20

func formatTokenCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// RenderReport renders a bar-chart usage report, used by both the chat
// TUI's /usage command and the monitor's usage panel.
func RenderReport(summaries []Summary, title string) string {
	if len(summaries) == 0 {
		return title + "\n\nNo usage recorded yet."
	}

	maxTokens := 0
	for _, s := range summaries {
		if t := s.TotalTokens(); t > maxTokens {
			maxTokens = t
		}
	}

	var b strings.Builder
	b.WriteString(title + "\n\n")

	var totalReq, totalPrompt, totalCompletion int
	var totalCost float64
	anyKnownCost := false

	for _, s := range summaries {
		totalReq += s.Requests
		totalPrompt += s.PromptTokens
		totalCompletion += s.CompletionTokens

		filled := 0
		if maxTokens > 0 {
			filled = s.TotalTokens() * reportBarWidth / maxTokens
		}
		if filled == 0 && s.TotalTokens() > 0 {
			filled = 1
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", reportBarWidth-filled)

		label := tiers.HeaderLabel(s.Tag)
		costStr := "n/a"
		if cost, ok := tiers.EstimatedCost(s.Tag, s.PromptTokens, s.CompletionTokens); ok {
			costStr = fmt.Sprintf("~$%.4f", cost)
			totalCost += cost
			anyKnownCost = true
		}

		fmt.Fprintf(&b, "%-32s %3d req  %6s tok  %s  %5.1f tok/s  %s\n",
			label, s.Requests, formatTokenCount(s.TotalTokens()), bar, s.TokensPerSec(), costStr)
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "Total: %d requests, %s tokens (%s prompt / %s completion)\n",
		totalReq, formatTokenCount(totalPrompt+totalCompletion), formatTokenCount(totalPrompt), formatTokenCount(totalCompletion))

	if anyKnownCost {
		fmt.Fprintf(&b, "Estimated equivalent cloud cost: ~$%.4f — actual cost running locally: $0.00\n", totalCost)
	} else {
		b.WriteString("Actual cost running locally: $0.00\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
