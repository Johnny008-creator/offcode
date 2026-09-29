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

func Resolve(backendFlag, baseURLFlag, model, modeFlag string) (Config, error) {
	mode := permission.Mode(modeFlag)
	switch mode {
	case permission.ModePlan, permission.ModeDefault, permission.ModeAuto:
	default:
		return Config{}, fmt.Errorf("invalid mode %q (expected plan, default, or auto)", modeFlag)
	}

	baseURL := baseURLFlag
	if baseURL == "" {
		preset, ok := presetBaseURLs[backendFlag]
		if !ok {
			return Config{}, fmt.Errorf("unknown backend %q (expected ollama or lmstudio), or pass -base-url explicitly", backendFlag)
		}
		baseURL = preset
	}

	if model == "" {
		return Config{}, fmt.Errorf("-model is required (e.g. qwen2.5-coder:32b)")
	}

	return Config{Backend: backendFlag, BaseURL: baseURL, Model: model, Mode: mode}, nil
}
