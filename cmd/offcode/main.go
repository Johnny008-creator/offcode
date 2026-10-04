package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/monitor"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "monitor":
			runMonitor(os.Args[2:])
			return
		case "models":
			runModels(os.Args[2:])
			return
		case "doctor":
			runDoctor(os.Args[2:])
			return
		case "version", "-v", "--version":
			runVersion()
			return
		}
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

func runVersion() {
	fmt.Printf("offcode v%s\n", tui.AppVersion)
}

func runModels(args []string) {
	fs := flag.NewFlagSet("offcode models", flag.ExitOnError)
	backendFlag := fs.String("backend", "ollama", "backend to use: ollama or lmstudio")
	baseURLFlag := fs.String("base-url", "", "override the backend's OpenAI-compatible base URL")
	fs.Parse(args)

	baseURL, err := config.ResolveBaseURL(*backendFlag, *baseURLFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode models:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := backend.NewClient(baseURL)
	installed, err := client.ListModels(ctx, *backendFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode models: could not reach backend:", err)
		os.Exit(1)
	}

	installedSet := map[string]bool{}
	for _, tag := range installed {
		installedSet[tag] = true
	}

	fmt.Println("offcode tiers:")
	for _, t := range tiers.Known {
		mark := "not installed"
		if installedSet[t.Tag] {
			mark = "installed"
		}
		fmt.Printf("  %-16s %-14s %-40s [%s]\n", t.DisplayName, t.Tag, t.Tagline, mark)
	}

	var other []string
	for _, tag := range installed {
		if _, known := tiers.For(tag); !known {
			other = append(other, tag)
		}
	}
	if len(other) > 0 {
		fmt.Println("\nOther installed models:")
		for _, tag := range other {
			fmt.Printf("  %s\n", tag)
		}
	}
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("offcode doctor", flag.ExitOnError)
	fixFlag := fs.Bool("fix", false, "automatically run `ollama pull` for any missing tier model")
	fs.Parse(args)

	ok := true
	check := func(name string, passed bool, detail string) {
		mark := "✓"
		if !passed {
			mark = "✗"
			ok = false
		}
		fmt.Printf("%s %-28s %s\n", mark, name, detail)
	}

	httpClient := &http.Client{Timeout: 3 * time.Second}

	resp, err := httpClient.Get("http://localhost:11434/api/version")
	if err != nil {
		check("Ollama reachable", false, "http://localhost:11434 — "+err.Error())
	} else {
		resp.Body.Close()
		check("Ollama reachable", resp.StatusCode == http.StatusOK, "http://localhost:11434")
	}

	resp, err = httpClient.Get("http://127.0.0.1:8888/search?q=test&format=json")
	if err != nil {
		check("SearXNG reachable (web_search)", false, "http://127.0.0.1:8888 — "+err.Error())
	} else {
		resp.Body.Close()
		check("SearXNG reachable (web_search)", resp.StatusCode == http.StatusOK, "http://127.0.0.1:8888")
	}

	var missing []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := backend.NewClient("http://localhost:11434/v1")
	installed, err := client.ListModels(ctx, "ollama")
	if err != nil {
		check("Model tiers installed", false, "could not list Ollama models: "+err.Error())
	} else {
		installedSet := map[string]bool{}
		for _, tag := range installed {
			installedSet[tag] = true
		}
		for _, t := range tiers.Known {
			check(t.DisplayName+" ("+t.Tag+")", installedSet[t.Tag], "ollama pull "+t.Tag)
			if !installedSet[t.Tag] {
				missing = append(missing, t.Tag)
			}
		}
	}

	if *fixFlag && len(missing) > 0 {
		fmt.Println()
		fmt.Printf("--fix: pulling %d missing model(s)...\n", len(missing))
		for _, tag := range missing {
			fmt.Printf("\n$ ollama pull %s\n", tag)
			cmd := exec.Command("ollama", "pull", tag)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "offcode doctor: pulling %s failed: %v\n", tag, err)
				os.Exit(1)
			}
		}
		fmt.Println("\nAll missing models pulled.")
		return
	}

	fmt.Println()
	switch {
	case ok:
		fmt.Println("All checks passed.")
	case len(missing) > 0 && !*fixFlag:
		fmt.Println("Some checks failed — see above. Run `offcode doctor --fix` to pull missing models automatically.")
		os.Exit(1)
	default:
		fmt.Println("Some checks failed — see above.")
		os.Exit(1)
	}
}
