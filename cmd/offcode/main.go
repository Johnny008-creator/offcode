package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/media"
	"github.com/Johnny008-creator/offcode/internal/monitor"
	"github.com/Johnny008-creator/offcode/internal/setup"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/tui"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
	"github.com/Johnny008-creator/offcode/internal/webserver"
)

func main() {
	refreshOllamaEnvironment()
	settings, setupErr := setup.Load()
	if setupErr == nil {
		// Setup must see the original environment, not a previously restored snapshot.
		if len(os.Args) < 2 || os.Args[1] != "setup" {
			setup.Apply(settings)
		}
	} else {
		fmt.Fprintln(os.Stderr, "offcode: invalid setup preferences:", setupErr)
	}
	restarted := os.Getenv("OFFCODE_UPDATE_RESTART") == tui.AppVersion
	os.Unsetenv("OFFCODE_UPDATE_RESTART")
	if !restarted && (len(os.Args) == 1 || os.Args[1] == "web" || (strings.HasPrefix(os.Args[1], "-") && os.Args[1] != "--version" && os.Args[1] != "-v" && os.Args[1] != "-h" && os.Args[1] != "--help")) {
		startupUpdate()
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup(os.Args[2:])
			return
		case "update":
			runUpdate(os.Args[2:])
			return
		case "eval-case":
			runEvalCase(os.Args[2:])
			return
		case "tool":
			runTool(os.Args[2:])
			return
		case "learn":
			runLearn(os.Args[2:])
			return
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
		case "web":
			runWeb(os.Args[2:])
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
	resumeFlag := fs.Bool("resume", false, "pick a saved session for this directory to continue, instead of starting new")
	fs.Parse(args)

	cfg, err := config.Resolve(*backendFlag, *baseURLFlag, *modelFlag, *modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		fs.Usage()
		os.Exit(1)
	}
	cfg.Resume = *resumeFlag

	ensureOllamaRunning(cfg)
	if err := tui.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "offcode:", err)
		os.Exit(1)
	}
}

func runWeb(args []string) {
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

	fs := flag.NewFlagSet("offcode web", flag.ExitOnError)
	portFlag := fs.Int("port", 7878, "port to listen on (binds to 127.0.0.1 only)")
	backendFlag := fs.String("backend", def.Backend, "backend to use: ollama or lmstudio")
	baseURLFlag := fs.String("base-url", def.BaseURL, "override the backend's OpenAI-compatible base URL")
	modelFlag := fs.String("model", def.Model, "model name as known to the backend; omit to pick from the browser")
	modeFlag := fs.String("mode", def.Mode, "permission mode: plan, default, or auto")
	resumeFlag := fs.Bool("resume", false, "attach to this directory's most recently saved session instead of starting new (the browser sidebar can switch to any session regardless)")
	fs.Parse(args)

	cfg, err := config.Resolve(*backendFlag, *baseURLFlag, *modelFlag, *modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "offcode web:", err)
		fs.Usage()
		os.Exit(1)
	}
	cfg.Resume = *resumeFlag

	ensureOllamaRunning(cfg)
	addr := fmt.Sprintf("127.0.0.1:%d", *portFlag)
	if err := webserver.Serve(addr, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "offcode web:", err)
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
		fmt.Printf("pdf-engine: %s\n", config.PDFEngine())
		fmt.Printf("whisper-model: %s\n", orNone(config.WhisperModel()))

	case "set":
		fs := flag.NewFlagSet("offcode config set", flag.ExitOnError)
		backendFlag := fs.String("backend", "", "save as default backend")
		modelFlag := fs.String("model", "", "save as default model")
		modeFlag := fs.String("mode", "", "save as default permission mode")
		baseURLFlag := fs.String("base-url", "", "save as default base URL")
		pdfEngineFlag := fs.String("pdf-engine", "", "PDF engine executable")
		whisperModelFlag := fs.String("whisper-model", "", "local whisper.cpp model path")
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
		if *pdfEngineFlag != "" {
			fc.PDFEngine = *pdfEngineFlag
		}
		if *whisperModelFlag != "" {
			fc.WhisperModel = *whisperModelFlag
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
	extensionsFlag := fs.Bool("extensions", false, "also check document, voice and media dependencies")
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
	fc, err := config.LoadFile()
	if err != nil {
		failExtra(err)
	}
	configuredURL := ""
	if fc.Backend == "ollama" || fc.Backend == "" {
		configuredURL = fc.BaseURL
	}
	baseURL, err := config.ResolveBaseURL("ollama", configuredURL)
	if err != nil {
		failExtra(err)
	}
	root := strings.TrimSuffix(baseURL, "/v1")
	resp, err := httpClient.Get(root + "/api/version")
	if err != nil {
		check("Ollama reachable", false, root+" — "+err.Error())
	} else {
		resp.Body.Close()
		check("Ollama reachable", resp.StatusCode == http.StatusOK, root)
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
	if loaded, err := httpClient.Get(root + "/api/ps"); err == nil {
		if loaded.StatusCode == http.StatusOK {
			var status struct {
				Models []struct {
					Name string
					Size int64
					VRAM int64 `json:"size_vram"`
				}
			}
			if json.NewDecoder(loaded.Body).Decode(&status) == nil {
				if len(status.Models) == 0 {
					fmt.Println("  GPU placement: no model loaded; send a message, then rerun doctor.")
				}
				for _, m := range status.Models {
					processor := "CPU only"
					if m.VRAM > 0 {
						processor = "GPU/mixed"
					}
					fmt.Printf("  %s: %s, VRAM %.2f GiB / total %.2f GiB\n", m.Name, processor, float64(m.VRAM)/(1<<30), float64(m.Size)/(1<<30))
				}
			}
		}
		loaded.Body.Close()
	}
	client := backend.NewClient(baseURL)
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

	if *extensionsFlag {
		engine := config.PDFEngine()
		for _, name := range []string{"pandoc", engine, "ffmpeg", "whisper-cli", "espeak-ng"} {
			path, err := exec.LookPath(name)
			if err != nil {
				check(name, false, "not found on PATH")
			} else {
				check(name, true, path)
			}
		}
		model := config.WhisperModel()
		info, err := os.Stat(model)
		check("Whisper model", err == nil && info != nil && !info.IsDir(), model)
		cfg, err := media.Load()
		if err != nil {
			check("ComfyUI configuration", false, err.Error())
		} else {
			for _, kind := range []string{"image", "video", "music"} {
				workflow, exists := cfg.Workflows[kind]
				_, err := os.Stat(workflow.File)
				check(kind+" workflow", exists && err == nil, workflow.File)
			}
			resp, err := httpClient.Get(cfg.URL + "/system_stats")
			if err != nil {
				check("ComfyUI reachable", false, err.Error())
			} else {
				resp.Body.Close()
				check("ComfyUI reachable", resp.StatusCode == 200, cfg.URL)
			}
		}
	}

	if *fixFlag && len(missing) > 0 {
		fmt.Println()
		fmt.Printf("--fix: pulling %d missing model(s)...\n", len(missing))
		for _, tag := range missing {
			fmt.Printf("\n$ ollama pull %s\n", tag)
			cmd := exec.Command("ollama", "pull", tag)
			cmd.Env = append(os.Environ(), "OLLAMA_HOST="+root)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "offcode doctor: pulling %s failed: %v\n", tag, err)
				os.Exit(1)
			}
		}
		fmt.Println("\nAll missing models pulled. Rechecking services.")
		verifyArgs := []string{}
		if *extensionsFlag {
			verifyArgs = append(verifyArgs, "-extensions")
		}
		runDoctor(verifyArgs)
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
