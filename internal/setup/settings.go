// Package setup stores installer preferences separately from chat defaults.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	OllamaMode string            `json:"ollama_mode"`
	OllamaEnv  map[string]string `json:"ollama_env,omitempty"`
	Preserve   []string          `json:"preserve"` // nil migrates legacy all-or-nothing settings; [] means none.
	Updates    string            `json:"updates"`
}

type PreserveGroup struct {
	ID    string
	Label string
	Keys  []string
}

var PreserveGroups = []PreserveGroup{
	{"server", "Server address", []string{"OLLAMA_HOST"}},
	{"gpu", "GPU selection and acceleration", []string{"OLLAMA_VULKAN", "OLLAMA_LLM_LIBRARY", "OLLAMA_GPU_OVERHEAD", "OLLAMA_SCHED_SPREAD", "OLLAMA_INTEL_GPU", "OLLAMA_IGPU_ENABLE", "CUDA_VISIBLE_DEVICES", "HIP_VISIBLE_DEVICES", "ROCR_VISIBLE_DEVICES", "GPU_DEVICE_ORDINAL", "GGML_VK_VISIBLE_DEVICES", "HSA_OVERRIDE_GFX_VERSION"}},
	{"models", "Model storage directory", []string{"OLLAMA_MODELS"}},
	{"context", "Context size", []string{"OLLAMA_CONTEXT_LENGTH"}},
	{"parallel", "Parallel requests and loaded-model limits", []string{"OLLAMA_NUM_PARALLEL", "OLLAMA_MAX_LOADED_MODELS"}},
	{"cache", "Flash Attention and KV cache", []string{"OLLAMA_FLASH_ATTENTION", "OLLAMA_KV_CACHE_TYPE"}},
	{"keepalive", "Time models stay loaded", []string{"OLLAMA_KEEP_ALIVE"}},
}

var EnvironmentKeys = func() []string {
	var keys []string
	for _, g := range PreserveGroups {
		keys = append(keys, g.Keys...)
	}
	return keys
}()

func (s Settings) Keeps(id string) bool {
	if s.Preserve == nil {
		return s.OllamaMode != "auto"
	}
	for _, selected := range s.Preserve {
		if selected == id {
			return true
		}
	}
	return false
}
func (s Settings) Selection() string {
	var ids []string
	for _, g := range PreserveGroups {
		if s.Keeps(g.ID) {
			ids = append(ids, g.ID)
		}
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ",")
}
func ParsePreserve(input string) ([]string, error) {
	selected := []string{}
	input = strings.TrimSpace(input)
	if input == "none" {
		return selected, nil
	}
	if input == "all" {
		for _, g := range PreserveGroups {
			selected = append(selected, g.ID)
		}
		return selected, nil
	}
	seen := map[string]bool{}
	for _, id := range strings.Split(input, ",") {
		id = strings.TrimSpace(id)
		valid := false
		for _, g := range PreserveGroups {
			if g.ID == id {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("unknown preservation choice %q", id)
		}
		if !seen[id] {
			selected = append(selected, id)
			seen[id] = true
		}
	}
	return selected, nil
}

// Retain only checked groups, preferring current values to an earlier snapshot.
func (s *Settings) Capture() {
	env := map[string]string{}
	for _, g := range PreserveGroups {
		if s.Keeps(g.ID) {
			for _, k := range g.Keys {
				if v, ok := os.LookupEnv(k); ok {
					env[k] = v
				} else if v, ok := s.OllamaEnv[k]; ok {
					env[k] = v
				}
			}
		}
	}
	s.OllamaEnv = env
}

func Load() (Settings, error) {
	s := Settings{OllamaMode: "preserve", Updates: "auto"}
	p, err := path()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	if err == nil {
		err = Validate(s)
	}
	return s, err
}
func Validate(s Settings) error {
	for _, id := range s.Preserve {
		valid := false
		for _, g := range PreserveGroups {
			if g.ID == id {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unknown preservation choice %q", id)
		}
	}
	if s.OllamaMode != "preserve" && s.OllamaMode != "auto" && s.OllamaMode != "external" {
		return fmt.Errorf("invalid Ollama mode %q", s.OllamaMode)
	}
	if s.Updates != "auto" && s.Updates != "notify" && s.Updates != "off" {
		return fmt.Errorf("invalid update mode %q", s.Updates)
	}
	return nil
}
func path() (string, error) {
	h, err := os.UserHomeDir()
	return filepath.Join(h, ".offcode", "setup.json"), err
}
func Save(s Settings) error {
	if err := Validate(s); err != nil {
		return err
	}
	p, err := path()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0600)
}

// Current environment wins over saved fallback values. Unchecked groups are
// neither captured nor restored; system/service configuration is never erased.
func Apply(s Settings) {
	for _, g := range PreserveGroups {
		if !s.Keeps(g.ID) {
			continue
		}
		for _, k := range g.Keys {
			if _, ok := os.LookupEnv(k); !ok {
				if v, exists := s.OllamaEnv[k]; exists {
					_ = os.Setenv(k, v)
				}
			}
		}
	}
	if s.Keeps("context") {
		if _, ok := os.LookupEnv("OFFCODE_NUM_CTX"); !ok {
			_ = os.Setenv("OFFCODE_NUM_CTX", "0")
		}
	}
}
