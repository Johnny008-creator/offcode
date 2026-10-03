// Package tiers maps known backend model tags to offcode's own branded
// tier identity, shared between the chat TUI and the monitor so both show
// the same names. The display name and tagline are what the user sees in
// the UI (pickers, header, monitor); the underlying tag's vendor/family is
// never surfaced in UI copy or the system prompt — the model itself is
// told its offcode name as its identity, not "qwen" or "gemma".
package tiers

type ModelTier struct {
	Tag         string
	DisplayName string
	Tagline     string

	// PromptPer1M/CompletionPer1M are illustrative USD-per-1M-token
	// reference rates for what a comparably-capable tier would roughly
	// cost on a metered cloud API — NOT a quote from any specific
	// provider, and not what offcode actually charges (that's $0.00: it
	// runs on the user's own hardware). Exists purely so usage views can
	// show a "for comparison" cost alongside real token counts.
	PromptPer1M     float64
	CompletionPer1M float64
}

var Known = []ModelTier{
	{Tag: "qwen3:1.7b", DisplayName: "Offcode Spark", Tagline: "ultra-lite — fastest, simple tasks", PromptPer1M: 0.05, CompletionPer1M: 0.15},
	{Tag: "qwen3.5:4b", DisplayName: "Offcode Ember", Tagline: "lite — quick everyday coding", PromptPer1M: 0.15, CompletionPer1M: 0.60},
	{Tag: "qwen3:8b", DisplayName: "Offcode Forge", Tagline: "medium — balanced default", PromptPer1M: 0.30, CompletionPer1M: 1.20},
	{Tag: "gemma4:12b", DisplayName: "Offcode Anvil", Tagline: "high — most capable, slower", PromptPer1M: 0.60, CompletionPer1M: 2.40},
}

// EstimatedCost computes the illustrative equivalent-cloud-cost for a
// token count, if the tag is a known tier.
func EstimatedCost(tag string, promptTokens, completionTokens int) (cost float64, known bool) {
	t, ok := For(tag)
	if !ok {
		return 0, false
	}
	cost = float64(promptTokens)/1_000_000*t.PromptPer1M + float64(completionTokens)/1_000_000*t.CompletionPer1M
	return cost, true
}

func For(tag string) (ModelTier, bool) {
	for _, t := range Known {
		if t.Tag == tag {
			return t, true
		}
	}
	return ModelTier{}, false
}

// Label formats a model tag for display, with its offcode tier identity
// and tagline when known.
func Label(tag string) string {
	if t, ok := For(tag); ok {
		return t.DisplayName + " (" + tag + ") — " + t.Tagline
	}
	return tag
}

// HeaderLabel is a shorter form for single-line contexts, without the
// tagline.
func HeaderLabel(tag string) string {
	if t, ok := For(tag); ok {
		return t.DisplayName + " (" + tag + ")"
	}
	return tag
}
