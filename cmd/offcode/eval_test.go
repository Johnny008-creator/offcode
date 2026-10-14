package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvalWriteAllowed(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"notes.html":                          true,
		"site/index.html":                     true,
		filepath.Join(cwd, "notes.html"):      true,
		filepath.Join(cwd, "..", "x.html"):    false,
		"../x.html":                           false,
		filepath.Join(cwd, ".offcode", "x"):   false,
		".offcode/config.json":                false,
		filepath.Join(filepath.Dir(cwd), "x"): false,
	}
	for path, want := range cases {
		if got := evalWriteAllowed(path); got != want {
			t.Errorf("evalWriteAllowed(%q)=%v want %v", path, got, want)
		}
	}
}
