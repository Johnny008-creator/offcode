package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/monitor"
	"github.com/Johnny008-creator/offcode/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		runMonitor(os.Args[2:])
		return
	}
	runChat(os.Args[1:])
}

func runChat(args []string) {
	fs := flag.NewFlagSet("offcode", flag.ExitOnError)
	backendFlag := fs.String("backend", "ollama", "backend to use: ollama or lmstudio")
	baseURLFlag := fs.String("base-url", "", "override the backend's OpenAI-compatible base URL")
	modelFlag := fs.String("model", "", "model name as known to the backend (e.g. qwen2.5-coder:32b)")
	modeFlag := fs.String("mode", "default", "permission mode: plan, default, or auto")
	fs.Parse(args)

	cfg, err := config.Resolve(*backendFlag, *baseURLFlag, *modelFlag, *modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		fs.Usage()
		os.Exit(1)
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		os.Exit(1)
	}
}

func runMonitor(args []string) {
	fs := flag.NewFlagSet("offcode monitor", flag.ExitOnError)
	rootFlag := fs.String("ollama-url", "http://localhost:11434", "Ollama server root URL")
	fs.Parse(args)

	if err := monitor.Run(*rootFlag); err != nil {
		fmt.Fprintln(os.Stderr, "offcode monitor:", err)
		os.Exit(1)
	}
}
