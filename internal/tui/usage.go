package tui

import (
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
)

func recordUsage(tag string, usage backend.Usage, wallClock time.Duration) {
	genDur := wallClock
	if usage.EvalDurationNs > 0 {
		// Prefer pure generation time over wall-clock (which includes
		// prompt processing) for an honest tok/s in usage reports.
		genDur = time.Duration(usage.EvalDurationNs)
	}
	// Best-effort: a failure to persist usage should never interrupt the
	// chat loop, so the error is intentionally dropped here.
	_ = usagelog.Append(usagelog.Record{
		Time:             time.Now(),
		Model:            tag,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		DurationMs:       genDur.Milliseconds(),
	})
}

func (m Model) renderUsageReport() string {
	records, err := usagelog.ReadAll()
	if err != nil {
		return "could not read usage log: " + err.Error()
	}
	summaries := usagelog.Summarize(records, m.sessionStart)
	return usagelog.RenderReport(summaries, "Usage this session")
}
