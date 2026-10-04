package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/monitor"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/tui"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
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
		case "usage":
			runUsage(os.Args[2:])
			return
		case "config":
			runConfig(os.Args[2:])
			return
		}
	}
	runChat(os.Args[1:])
}

// chatDefaults are the hardcoded fallbacks when neither a CLI flag nor the
// saved ~/.offcode/config.json has an opinion.
var chatDefaults = config.FileConfig{Backend: "ollama", Mode: "default"}

func runChat(args []string) {
	saved, err := config.LoadFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode: warning: couldn't read ~/.offcode/config.json:", err)
	}
	def := chatDefaults
	if saved.Backend != "" {
		def.Backend = saved.Backend
	}
	if saved.Model != "" {
		def.Model = saved.Model
	}
	if saved.Mode != "" {
		def.Mode = saved.Mode
	}
	if saved.BaseURL != "" {
		def.BaseURL = saved.BaseURL
	}

	fs := flag.NewFlagSet("offcode", flag.ExitOnError)
	backendFlag := fs.String("backend", def.Backend, "backend to use: ollama or lmstudio")
	baseURLFlag := fs.String("base-url", def.BaseURL, "override the backend's OpenAI-compatible base URL")
	modelFlag := fs.String("model", def.Model, "model name as known to the backend (e.g. qwen2.5-coder:32b)")
	modeFlag := fs.String("mode", def.Mode, "permission mode: plan, default, or auto")
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

func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: offcode config show | offcode config set [-backend=X] [-model=Y] [-mode=Z] [-base-url=W]")
		os.Exit(1)
	}

	switch args[0] {
	case "show":
		fc, err := config.LoadFile()
		if err != nil {
			fmt.Fprintln(os.Stderr, "offcode config:", err)
			os.Exit(1)
		}
		fmt.Printf("backend:  %s\n", orNone(fc.Backend))
		fmt.Printf("model:    %s\n", orNone(fc.Model))
		fmt.Printf("mode:     %s\n", orNone(fc.Mode))
		fmt.Printf("base-url: %s\n", orNone(fc.BaseURL))

	case "set":
		fs := flag.NewFlagSet("offcode config set", flag.ExitOnError)
		backendFlag := fs.String("backend", "", "save as default backend")
		modelFlag := fs.String("model", "", "save as default model")
		modeFlag := fs.String("mode", "", "save as default permission mode")
		baseURLFlag := fs.String("base-url", "", "save as default base URL")
		fs.Parse(args[1:])

		fc, err := config.LoadFile()
		if err != nil {
			fmt.Fprintln(os.Stderr, "offcode config:", err)
			os.Exit(1)
		}
		if *backendFlag != "" {
			fc.Backend = *backendFlag
		}
		if *modelFlag != "" {
			fc.Model = *modelFlag
		}
		if *modeFlag != "" {
			fc.Mode = *modeFlag
		}
		if *baseURLFlag != "" {
			fc.BaseURL = *baseURLFlag
		}
		if err := config.SaveFile(fc); err != nil {
			fmt.Fprintln(os.Stderr, "offcode config:", err)
			os.Exit(1)
		}
		fmt.Println("saved to ~/.offcode/config.json")

	default:
		fmt.Fprintln(os.Stderr, "offcode config: unknown subcommand", args[0], "(expected show or set)")
		os.Exit(1)
	}
}

func orNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func runUsage(args []string) {
	fs := flag.NewFlagSet("offcode usage", flag.ExitOnError)
	csvFlag := fs.Bool("csv", false, "export raw records as CSV instead of the summary report")
	fs.Parse(args)

	records, err := usagelog.ReadAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode usage:", err)
		os.Exit(1)
	}

	if *csvFlag {
		w := csv.NewWriter(os.Stdout)
		w.Write([]string{"time", "model", "prompt_tokens", "completion_tokens", "duration_ms"})
		for _, r := range records {
			w.Write([]string{
				r.Time.Format(time.RFC3339),
				r.Model,
				strconv.Itoa(r.PromptTokens),
				strconv.Itoa(r.CompletionTokens),
				strconv.FormatInt(r.DurationMs, 10),
			})
		}
		w.Flush()
		return
	}

	summaries := usagelog.Summarize(records, time.Time{})
	fmt.Println(usagelog.RenderReport(summaries, "Usage (all-time)"))
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
