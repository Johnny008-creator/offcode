package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/setup"
	"github.com/Johnny008-creator/offcode/internal/tiers"
)

func runSetup(args []string) {
	if err := configureInstallation(args); err != nil {
		failExtra(err)
	}
}
func configureInstallation(args []string) error {
	saved, err := setup.Load()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("offcode setup", flag.ContinueOnError)
	interactive := fs.Bool("interactive", false, "ask for installation preferences")
	mode := fs.String("ollama", saved.OllamaMode, "preserve (keep GPU/context settings), auto (agent context), external (never start Ollama)")
	preserve := fs.String("preserve", saved.Selection(), "comma-separated server,gpu,models,context,parallel,cache,keepalive; all or none")
	updates := fs.String("updates", saved.Updates, "auto, notify, or off")
	models := fs.String("models", "none", "comma-separated spark,ember,forge,anvil,foundry,embedding or Ollama model tags; all or none")
	install := fs.Bool("install-ollama", false, "install missing Ollama using winget on Windows")
	if err = fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected setup arguments: %v", fs.Args())
	}
	preserveSet, modeSet := false, false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "preserve" {
			preserveSet = true
		}
		if f.Name == "ollama" {
			modeSet = true
		}
	})
	if modeSet && !preserveSet && (*mode == "auto" || saved.Preserve == nil) {
		legacy := setup.Settings{OllamaMode: *mode}
		*preserve = legacy.Selection()
	}
	choices, err := setup.ParsePreserve(*preserve)
	if err != nil {
		return err
	}
	if *interactive {
		choices, err = choosePreservation(choices)
		if err != nil {
			return err
		}

		in := bufio.NewReader(os.Stdin)
		ask := func(prompt, def string) (string, error) {
			fmt.Printf("%s [%s]: ", prompt, def)
			line, err := in.ReadString('\n')
			if err != nil && err != io.EOF {
				return "", err
			}
			if err == io.EOF && line == "" {
				return "", fmt.Errorf("setup input closed; use explicit flags for unattended installation")
			}
			line = strings.TrimSpace(line)
			if line == "" {
				line = def
			}
			return line, nil
		}
		fmt.Println("Offcode setup — existing Ollama installations and model files are never reset.")
		if *mode, err = ask("Ollama startup: preserve (start if needed) or external (existing server only)", *mode); err != nil {
			return err
		}
		if *updates, err = ask("Startup updates: auto, notify, off", *updates); err != nil {
			return err
		}
		for _, t := range tiers.Known {
			fmt.Printf("  %s: %s (%s)\n", strings.TrimPrefix(strings.ToLower(t.DisplayName), "offcode "), t.Tag, t.Tagline)
		}
		fmt.Println("  embedding: semantic search model; all: all five tiers (large downloads)")
		if *models, err = ask("Models to download (comma-separated names, all, none)", *models); err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			if _, err := exec.LookPath("ollama"); err != nil {
				answer, e := ask("Install Ollama with winget if model download needs it? yes/no", "no")
				if e != nil {
					return e
				}
				*install = strings.EqualFold(answer, "yes")
			}
		}
	}
	selected, err := selectedModels(*models)
	if err != nil {
		return err
	}
	saved.OllamaMode = *mode
	saved.Preserve = choices
	saved.Updates = *updates
	if err = setup.Validate(saved); err != nil {
		return err
	}
	saved.Capture()
	if err = setup.Save(saved); err != nil {
		return err
	}
	setup.Apply(saved)
	fmt.Println("Preferences saved. System prompts, tier identities and skills are bundled in Offcode, including offline.")
	if len(selected) == 0 {
		return nil
	}
	fc, err := config.LoadFile()
	if err != nil {
		return err
	}
	base := ""
	if fc.Backend == "" || fc.Backend == "ollama" {
		base = fc.BaseURL
	}
	base, err = config.ResolveBaseURL("ollama", base)
	if err != nil {
		return err
	}
	root := strings.TrimSuffix(base, "/v1")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *install && !ollamaReachable(root) {
		if _, err := exec.LookPath("ollama"); err != nil {
			if runtime.GOOS != "windows" {
				return fmt.Errorf("install Ollama from https://ollama.com/download, then rerun offcode setup")
			}
			cmd := exec.CommandContext(ctx, "winget", "install", "--id", "Ollama.Ollama", "--exact", "--accept-package-agreements", "--accept-source-agreements")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err = cmd.Run(); err != nil {
				return fmt.Errorf("install Ollama: %w", err)
			}
			// winget cannot refresh this process's PATH; Ollama's standard user install location.
			os.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+os.Getenv("LOCALAPPDATA")+`\Programs\Ollama`)
		}
	}
	ensureOllamaRunning(config.Config{Backend: "ollama", BaseURL: base})
	if !ollamaReachable(root) {
		return fmt.Errorf("Ollama is unavailable at %s; start your configured Ollama and rerun offcode setup -models=%s", root, *models)
	}
	installed := map[string]bool{}
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	tags, listErr := backend.NewClient(base).ListModels(listCtx, "ollama")
	listCancel()
	if listErr != nil {
		return fmt.Errorf("list installed models: %w", listErr)
	}
	for _, tag := range tags {
		installed[tag] = true
		installed[strings.TrimSuffix(tag, ":latest")] = true
	}
	var failures []string
	for _, tag := range selected {
		if installed[tag] {
			fmt.Println("Already installed:", tag)
		} else {
			fmt.Println("Downloading", tag)
			if err = pullModel(ctx, root, tag, os.Stdout); err != nil {
				failures = append(failures, tag+": "+err.Error())
				if ctx.Err() != nil {
					break
				}
				continue
			}
		}
		if fc.Model == "" && tag != "nomic-embed-text" && (fc.Backend == "" || fc.Backend == "ollama") {
			fc.Model = tag
			fc.Backend = "ollama"
			if err = config.SaveFile(fc); err != nil {
				return err
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("some downloads failed (rerun setup to resume):\n%s", strings.Join(failures, "\n"))
	}
	fmt.Println("Selected models are ready. Run offcode or offcode web.")
	return nil
}
func selectedModels(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "none" || input == "" {
		return nil, nil
	}
	var result []string
	seen := map[string]bool{}
	add := func(tag string) {
		if !seen[tag] {
			result = append(result, tag)
			seen[tag] = true
		}
	}
	for _, name := range strings.Split(input, ",") {
		name = strings.TrimSpace(name)
		if name == "all" {
			for _, t := range tiers.Known {
				add(t.Tag)
			}
			continue
		}
		if name == "embedding" {
			add("nomic-embed-text")
			continue
		}
		if t, ok := tiers.ByShortName(name); ok {
			add(t.Tag)
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]*$`).MatchString(name) || name == "none" {
			return nil, fmt.Errorf("invalid model selection %q", name)
		}
		add(name)
	}
	return result, nil
}
func pullModel(ctx context.Context, root, tag string, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Hour)
	defer cancel()
	b, _ := json.Marshal(map[string]any{"model": tag, "stream": true})
	req, err := http.NewRequestWithContext(ctx, "POST", root+"/api/pull", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(resp.Body)
	last := ""
	lastPercent := int64(-1)
	for {
		var event struct {
			Status    string
			Error     string
			Total     int64
			Completed int64
		}
		if err = decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return fmt.Errorf("download ended before success")
			}
			return err
		}
		if event.Error != "" {
			return fmt.Errorf("%s", event.Error)
		}
		percent := int64(0)
		if event.Total > 0 {
			percent = event.Completed * 100 / event.Total
		}
		if event.Status != last || percent/10 != lastPercent/10 {
			fmt.Fprintf(out, "  %s", event.Status)
			if event.Total > 0 {
				fmt.Fprintf(out, " %d%%", percent)
			}
			fmt.Fprintln(out)
			last = event.Status
			lastPercent = percent
		}
		if event.Status == "success" {
			return nil
		}
	}
}
