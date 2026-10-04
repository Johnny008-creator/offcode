package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileConfig is the persisted default config at ~/.offcode/config.json.
// Every field is optional (zero value = "not set") — CLI flags always
// override it, and it only fills in gaps left by flags the user didn't
// pass, so `offcode` alone can work from saved defaults without the file
// having to specify everything.
type FileConfig struct {
	Backend string `json:"backend,omitempty"`
	Model   string `json:"model,omitempty"`
	Mode    string `json:"mode,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

func filePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".offcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadFile reads the saved default config. A missing file is not an
// error — it just returns a zero-value FileConfig, same as "nothing
// saved yet".
func LoadFile() (FileConfig, error) {
	path, err := filePath()
	if err != nil {
		return FileConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, err
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return FileConfig{}, err
	}
	return fc, nil
}

// SaveFile writes fc, merged over whatever was already saved (fields left
// zero in fc don't erase a previously-saved value — pass the full desired
// FileConfig if you want to clear something).
func SaveFile(fc FileConfig) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
