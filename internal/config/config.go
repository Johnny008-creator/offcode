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

	if model == "" {
		return Config{}, fmt.Errorf("-model is required (e.g. qwen2.5-coder:32b)")
	}

	return Config{Backend: backendFlag, BaseURL: baseURL, Model: model, Mode: mode}, nil
}
