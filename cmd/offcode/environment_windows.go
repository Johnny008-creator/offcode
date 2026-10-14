//go:build windows

package main

import (
	"github.com/Johnny008-creator/offcode/internal/setup"
	"golang.org/x/sys/windows/registry"
	"os"
)

// A terminal opened before the Ollama settings were saved may have an old
// environment. Fill missing values from Windows, without overriding explicit
// per-process values. User settings override machine settings.
func refreshOllamaEnvironment() {
	for _, name := range setup.EnvironmentKeys {
		if _, ok := os.LookupEnv(name); ok {
			continue
		}
		for _, scope := range []struct {
			root registry.Key
			path string
		}{
			{registry.CURRENT_USER, `Environment`},
			{registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`},
		} {
			key, err := registry.OpenKey(scope.root, scope.path, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			value, kind, err := key.GetStringValue(name)
			key.Close()
			if err == nil {
				if kind == registry.EXPAND_SZ {
					if expanded, e := registry.ExpandString(value); e == nil {
						value = expanded
					}
				}
				os.Setenv(name, value)
				break
			}
		}
	}
}
