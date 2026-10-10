package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Johnny008-creator/offcode/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/tools"
)

func failExtra(err error) { fmt.Fprintln(os.Stderr, "offcode:", err); os.Exit(1) }

// An explicit CLI tool invocation has the same authority as a shell command.
func runTool(args []string) {
	if len(args) != 2 {
		failExtra(fmt.Errorf("usage: offcode tool NAME '{JSON arguments}'"))
	}
	out, err := tools.Dispatch(args[0], args[1])
	if err != nil {
		failExtra(err)
	}
	fmt.Println(out)
}

func runLearn(args []string) {
	if len(args) > 0 && args[0] == "deploy" {
		runLearnDeploy(args[1:])
		return
	}
	if len(args) == 0 || args[0] != "add" {
		failExtra(fmt.Errorf("usage: offcode learn add -tier forge -prompt-file prompt.txt -answer-file corrected.txt"))
	}
	fs := flag.NewFlagSet("offcode learn add", flag.ExitOnError)
	tier := fs.String("tier", "forge", "dataset name (one dataset per model/tier)")
	promptPath := fs.String("prompt-file", "", "reviewed user prompt file")
	answerPath := fs.String("answer-file", "", "reviewed correct assistant response file")
	fs.Parse(args[1:])
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(*tier) {
		failExtra(fmt.Errorf("invalid tier name"))
	}
	prompt, err := os.ReadFile(*promptPath)
	if err != nil {
		failExtra(err)
	}
	answer, err := os.ReadFile(*answerPath)
	if err != nil {
		failExtra(err)
	}
	if strings.TrimSpace(string(prompt)) == "" || strings.TrimSpace(string(answer)) == "" {
		failExtra(fmt.Errorf("prompt and answer cannot be empty"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		failExtra(err)
	}
	dir := filepath.Join(home, ".offcode", "training")
	if err := os.MkdirAll(dir, 0700); err != nil {
		failExtra(err)
	}
	path := filepath.Join(dir, *tier+".jsonl")
	row := map[string]any{"reviewed": true, "created_at": time.Now().UTC(), "messages": []map[string]string{{"role": "user", "content": string(prompt)}, {"role": "assistant", "content": string(answer)}}}
	data, err := json.Marshal(row)
	if err != nil {
		failExtra(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		failExtra(err)
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		failExtra(err)
	}
	if closeErr != nil {
		failExtra(closeErr)
	}
	fmt.Println("Added reviewed training example to", path)
}

func runLearnDeploy(args []string) {
	fs := flag.NewFlagSet("offcode learn deploy", flag.ExitOnError)
	merged := fs.String("merged", "", "merged Safetensors directory produced by training")
	name := fs.String("model", "", "new Ollama model name")
	activate := fs.Bool("activate", false, "select the deployed model as offcode's default")
	fs.Parse(args)
	if *merged == "" || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]*$`).MatchString(*name) {
		failExtra(fmt.Errorf("provide -merged and a valid -model name"))
	}
	dir, err := filepath.Abs(*merged)
	if err != nil {
		failExtra(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Modelfile"))
	if err != nil {
		failExtra(err)
	}
	if strings.TrimSpace(string(data)) != "FROM ." {
		failExtra(fmt.Errorf("expected the training export's Modelfile containing FROM ."))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ollama", "create", *name, "-f", "Modelfile")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		failExtra(err)
	}
	if *activate {
		cfg, err := config.LoadFile()
		if err != nil {
			failExtra(err)
		}
		cfg.Backend = "ollama"
		cfg.Model = *name
		cfg.BaseURL = ""
		if host := os.Getenv("OLLAMA_HOST"); host != "" {
			if !strings.Contains(host, "://") {
				host = "http://" + host
			}
			cfg.BaseURL = strings.TrimRight(host, "/") + "/v1"
		}
		if err := config.SaveFile(cfg); err != nil {
			failExtra(err)
		}
	}
	fmt.Println("Deployed model:", *name)
}
