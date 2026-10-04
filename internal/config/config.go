package config

import (
	"fmt"

	"github.com/Johnny008-creator/offcode/internal/permission"
)

type Config struct {
	Backend string
	BaseURL string
	Model   string
	Mode    permission.Mode

	// Fresh skips resuming a saved session for the current working
	// directory, even if one exists.
	Fresh bool
}

var presetBaseURLs = map[string]string{
	"ollama":   "http://localhost:11434/v1",
	"lmstudio": "http://localhost:1234/v1",
}

// ResolveBaseURL applies the same backend -> default base URL logic Resolve
// uses, without requiring a model — for subcommands like `offcode models`
// or `offcode doctor` that need to reach a backend but aren't running a chat.
func ResolveBaseURL(backendFlag, baseURLFlag string) (string, error) {
	if baseURLFlag != "" {
		return baseURLFlag, nil
	}
	preset, ok := presetBaseURLs[backendFlag]
	if !ok {
		return "", fmt.Errorf("unknown backend %q (expected ollama or lmstudio), or pass -base-url explicitly", backendFlag)
	}
	return preset, nil
}

func Resolve(backendFlag, baseURLFlag, model, modeFlag string) (Config, error) {
	mode := permission.Mode(modeFlag)
	switch mode {
	case permission.ModePlan, permission.ModeDefault, permission.ModeAuto:
	default:
		return Config{}, fmt.Errorf("invalid mode %q (expected plan, default, or auto)", modeFlag)
	}

	baseURL, err := ResolveBaseURL(backendFlag, baseURLFlag)
	if err != nil {
		return Config{}, err
	}

	// Model is deliberately allowed to be empty here: the chat TUI opens
	// the model picker on startup when none is given, so `offcode` alone
	// (no flags at all) works — picking a model is a UI step, not
	// something the user must already know a tag for on the command line.

	return Config{Backend: backendFlag, BaseURL: baseURL, Model: model, Mode: mode}, nil
}
