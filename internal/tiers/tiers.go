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
}

var Known = []ModelTier{
	{Tag: "qwen3:1.7b", DisplayName: "Offcode Spark", Tagline: "ultra-lite — fastest, simple tasks"},
	{Tag: "qwen3.5:4b", DisplayName: "Offcode Ember", Tagline: "lite — quick everyday coding"},
	{Tag: "qwen3:8b", DisplayName: "Offcode Forge", Tagline: "medium — balanced default"},
	{Tag: "gemma4:12b", DisplayName: "Offcode Anvil", Tagline: "high — most capable, slower"},
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
