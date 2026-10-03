package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

type ModelUsage struct {
	Tag              string
	Requests         int
	PromptTokens     int
	CompletionTokens int
	GenerateDuration time.Duration
}

func (u ModelUsage) TotalTokens() int {
	return u.PromptTokens + u.CompletionTokens
}

func (u ModelUsage) TokensPerSec() float64 {
	secs := u.GenerateDuration.Seconds()
	if secs <= 0 {
		return 0
	}
	return float64(u.CompletionTokens) / secs
}

// tierPricing is an illustrative reference rate (USD per 1M tokens) for
// what a comparably-capable tier would roughly cost on a metered cloud
// API — NOT a quote from any specific provider, and not what offcode
// actually charges (it's $0.00: this runs on the user's own hardware).
// It exists purely so /usage can show a "for comparison" cost alongside
// real token counts, the way Claude Code shows cost for its own (actually
// billed) usage.
type tierPricing struct {
	PromptPer1M     float64
	CompletionPer1M float64
}

var pricingByTag = map[string]tierPricing{
	"qwen3:1.7b": {PromptPer1M: 0.05, CompletionPer1M: 0.15},
	"qwen3.5:4b": {PromptPer1M: 0.15, CompletionPer1M: 0.60},
	"qwen3:8b":   {PromptPer1M: 0.30, CompletionPer1M: 1.20},
	"gemma4:12b": {PromptPer1M: 0.60, CompletionPer1M: 2.40},
}

func (u ModelUsage) EstimatedCost() (cost float64, known bool) {
	p, ok := pricingByTag[u.Tag]
	if !ok {
		return 0, false
	}
	cost = float64(u.PromptTokens)/1_000_000*p.PromptPer1M + float64(u.CompletionTokens)/1_000_000*p.CompletionPer1M
	return cost, true
}

func (m *Model) recordUsage(tag string, usage backend.Usage, dur time.Duration) {
	u, ok := m.usageByModel[tag]
	if !ok {
		u = &ModelUsage{Tag: tag}
		m.usageByModel[tag] = u
	}
	u.Requests++
	u.PromptTokens += usage.PromptTokens
	u.CompletionTokens += usage.CompletionTokens
	u.GenerateDuration += dur
}

const usageBarWidth = 20

func (m Model) renderUsageReport() string {
	if len(m.usageByModel) == 0 {
		return "No usage yet this session."
	}

	tags := make([]string, 0, len(m.usageByModel))
	for tag := range m.usageByModel {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		return m.usageByModel[tags[i]].TotalTokens() > m.usageByModel[tags[j]].TotalTokens()
	})

	maxTokens := 0
	for _, tag := range tags {
		if t := m.usageByModel[tag].TotalTokens(); t > maxTokens {
			maxTokens = t
		}
	}

	var b strings.Builder
	b.WriteString("Usage this session\n\n")

	var totalReq, totalPrompt, totalCompletion int
	var totalCost float64
	anyKnownCost := false

	for _, tag := range tags {
		u := m.usageByModel[tag]
		totalReq += u.Requests
		totalPrompt += u.PromptTokens
		totalCompletion += u.CompletionTokens

		filled := 0
		if maxTokens > 0 {
			filled = u.TotalTokens() * usageBarWidth / maxTokens
		}
		if filled == 0 && u.TotalTokens() > 0 {
			filled = 1
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", usageBarWidth-filled)

		label := modelHeaderLabel(tag)
		costStr := "n/a"
		if cost, ok := u.EstimatedCost(); ok {
			costStr = fmt.Sprintf("~$%.4f", cost)
			totalCost += cost
			anyKnownCost = true
		}

		fmt.Fprintf(&b, "%-32s %3d req  %6s tok  %s  %.1f tok/s  %s\n",
			label, u.Requests, formatTokens(u.TotalTokens()), bar, u.TokensPerSec(), costStr)
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "Total: %d requests, %s tokens (%s prompt / %s completion)\n",
		totalReq, formatTokens(totalPrompt+totalCompletion), formatTokens(totalPrompt), formatTokens(totalCompletion))

	if anyKnownCost {
		fmt.Fprintf(&b, "Estimated equivalent cloud cost: ~$%.4f — actual cost running locally: $0.00\n", totalCost)
	} else {
		b.WriteString("Actual cost running locally: $0.00\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
