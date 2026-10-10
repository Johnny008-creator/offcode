// Package media executes user-configured ComfyUI API workflows locally.
package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Workflow struct {
	File        string `json:"file"`
	PromptNode  string `json:"prompt_node"`
	PromptInput string `json:"prompt_input"`
	OutputNode  string `json:"output_node"`
	OutputKey   string `json:"output_key"`
}
type Config struct {
	URL       string              `json:"url"`
	Workflows map[string]Workflow `json:"workflows"`
}
type Client struct {
	URL          string
	HTTP         *http.Client
	PollInterval time.Duration
}
type artifact struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(home, ".offcode", "media.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("configure %s first: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.URL == "" {
		c.URL = "http://127.0.0.1:8188"
	}
	for k, w := range c.Workflows {
		if !filepath.IsAbs(w.File) {
			w.File = filepath.Join(filepath.Dir(path), w.File)
			c.Workflows[k] = w
		}
	}
	return c, nil
}

func Generate(ctx context.Context, kind, prompt, path string) (string, error) {
	c, err := Load()
	if err != nil {
		return "", err
	}
	w, ok := c.Workflows[kind]
	if !ok {
		return "", fmt.Errorf("no %s workflow configured in ~/.offcode/media.json", kind)
	}
	client := Client{URL: c.URL, HTTP: &http.Client{Timeout: 2 * time.Minute}, PollInterval: time.Second}
	return client.Generate(ctx, w, prompt, path)
}

func (c Client) request(ctx context.Context, method, path string, payload any, result any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ComfyUI HTTP %d: %s", resp.StatusCode, text)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
}

func (c Client) Generate(ctx context.Context, w Workflow, prompt, output string) (string, error) {
	if strings.TrimSpace(prompt) == "" || output == "" {
		return "", fmt.Errorf("prompt and output path are required")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return "", fmt.Errorf("output must be a new file: %s", output)
	}
	data, err := os.ReadFile(w.File)
	if err != nil {
		return "", err
	}
	var graph map[string]struct {
		ClassType string         `json:"class_type"`
		Inputs    map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(data, &graph); err != nil {
		return "", fmt.Errorf("workflow must be exported in API format: %w", err)
	}
	node, ok := graph[w.PromptNode]
	if !ok || node.Inputs == nil {
		return "", fmt.Errorf("prompt node %q is missing", w.PromptNode)
	}
	if w.PromptInput == "" {
		w.PromptInput = "text"
	}
	if _, ok := node.Inputs[w.PromptInput]; !ok {
		return "", fmt.Errorf("prompt input %q is missing", w.PromptInput)
	}
	node.Inputs[w.PromptInput] = prompt
	graph[w.PromptNode] = node
	if _, ok := graph[w.OutputNode]; !ok || w.OutputKey == "" {
		return "", fmt.Errorf("configure an output_node and output_key (images, audio, or gifs) from the workflow history")
	}
	var queued struct {
		ID     string          `json:"prompt_id"`
		Errors json.RawMessage `json:"node_errors"`
	}
	if err := c.request(ctx, "POST", "/prompt", map[string]any{"prompt": graph}, &queued); err != nil {
		return "", err
	}
	if queued.ID == "" {
		return "", fmt.Errorf("ComfyUI rejected workflow: %s", queued.Errors)
	}
	type historyEntry struct {
		Outputs map[string]map[string]json.RawMessage `json:"outputs"`
		Status  struct {
			Completed bool            `json:"completed"`
			Status    string          `json:"status_str"`
			Messages  json.RawMessage `json:"messages"`
		} `json:"status"`
	}
	interval := c.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	for {
		var history map[string]historyEntry
		if err := c.request(ctx, "GET", "/history/"+url.PathEscape(queued.ID), nil, &history); err != nil {
			return "", fmt.Errorf("job %s: %w", queued.ID, err)
		}
		if entry, ok := history[queued.ID]; ok {
			if entry.Status.Status == "error" {
				return "", fmt.Errorf("ComfyUI job %s failed: %s", queued.ID, entry.Status.Messages)
			}
			if entry.Status.Completed || entry.Status.Status == "success" {
				var files []artifact
				if err := json.Unmarshal(entry.Outputs[w.OutputNode][w.OutputKey], &files); err != nil || len(files) == 0 {
					return "", fmt.Errorf("job %s completed without files at %s/%s", queued.ID, w.OutputNode, w.OutputKey)
				}
				file := files[0]
				if file.Filename == "" || !strings.EqualFold(filepath.Ext(file.Filename), filepath.Ext(output)) {
					return "", fmt.Errorf("output extension must match generated file %q", file.Filename)
				}
				if err := c.download(ctx, file, output); err != nil {
					return "", err
				}
				return output, nil
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", fmt.Errorf("job %s may still be queued/running in ComfyUI: %w", queued.ID, ctx.Err())
		case <-timer.C:
		}
	}
}

func (c Client) download(ctx context.Context, a artifact, output string) error {
	query := url.Values{"filename": {a.Filename}, "subfolder": {a.Subfolder}, "type": {a.Type}}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.URL, "/")+"/view?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("artifact download returned HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		file.Close()
		if !success {
			os.Remove(output)
		}
	}()
	const maxSize = 512 << 20
	n, err := io.Copy(file, io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return err
	}
	if n == 0 || n > maxSize {
		return fmt.Errorf("generated artifact is empty or exceeds 512 MiB")
	}
	if err := file.Close(); err != nil {
		return err
	}
	success = true
	return nil
}
