package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/tui"
)

func main() {
	backendFlag := flag.String("backend", "ollama", "backend to use: ollama or lmstudio")
	baseURLFlag := flag.String("base-url", "", "override the backend's OpenAI-compatible base URL")
	modelFlag := flag.String("model", "", "model name as known to the backend (e.g. qwen2.5-coder:32b)")
	modeFlag := flag.String("mode", "default", "permission mode: plan, default, or auto")
	flag.Parse()

	cfg, err := config.Resolve(*backendFlag, *baseURLFlag, *modelFlag, *modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		flag.Usage()
		os.Exit(1)
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		os.Exit(1)
	}
}
