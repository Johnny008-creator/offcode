package tui

// ModelTier maps a known backend model tag to offcode's own branded tier
// identity. The display name and tagline are what the user sees in the UI
// (pickers, header); the underlying tag's vendor/family is never surfaced
// in the UI copy or the system prompt — the model itself is told this name
// as its identity, not "qwen" or "gemma".
type ModelTier struct {
	Tag         string
	DisplayName string
	Tagline     string
}

var knownTiers = []ModelTier{
	{Tag: "qwen3:1.7b", DisplayName: "Offcode Spark", Tagline: "ultra-lite — fastest, simple tasks"},
	{Tag: "qwen3.5:4b", DisplayName: "Offcode Ember", Tagline: "lite — quick everyday coding"},
	{Tag: "qwen3:8b", DisplayName: "Offcode Forge", Tagline: "medium — balanced default"},
	{Tag: "gemma4:12b", DisplayName: "Offcode Anvil", Tagline: "high — most capable, slower"},
}

func tierFor(tag string) (ModelTier, bool) {
	for _, t := range knownTiers {
		if t.Tag == tag {
			return t, true
		}
	}
	return ModelTier{}, false
}

// modelLabel formats a model tag for display, prefixed with its offcode
// tier identity when known.
func modelLabel(tag string) string {
	if t, ok := tierFor(tag); ok {
		return t.DisplayName + " (" + tag + ") — " + t.Tagline
	}
	return tag
}

// modelHeaderLabel is a shorter form for the single-line header, without
// the tagline.
func modelHeaderLabel(tag string) string {
	if t, ok := tierFor(tag); ok {
		return t.DisplayName + " (" + tag + ")"
	}
	return tag
}
