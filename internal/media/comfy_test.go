package media

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestWorkflowQueueHistoryDownload(t *testing.T) {
	dir := t.TempDir()
	workflow := filepath.Join(dir, "workflow.json")
	out := filepath.Join(dir, "result.png")
	if err := os.WriteFile(workflow, []byte(`{"6":{"class_type":"CLIPTextEncode","inputs":{"text":"old"}},"9":{"class_type":"SaveImage","inputs":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	c := Client{URL: "http://comfy.test", PollInterval: time.Millisecond, HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		requests++
		switch r.URL.Path {
		case "/prompt":
			var body map[string]map[string]struct {
				Inputs map[string]any `json:"inputs"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["prompt"]["6"].Inputs["text"] != "quoted \"prompt\" česky" {
				t.Fatalf("prompt not injected safely: %v", body)
			}
			return reply(`{"prompt_id":"job1"}`), nil
		case "/history/job1":
			return reply(`{"job1":{"status":{"completed":true,"status_str":"success"},"outputs":{"9":{"images":[{"filename":"out.png","subfolder":"","type":"output"}]}}}}`), nil
		case "/view":
			if r.URL.Query().Get("filename") != "out.png" {
				t.Fatal("wrong file requested")
			}
			return reply("PNG DATA"), nil
		default:
			t.Fatalf("unexpected URL %s", r.URL)
			return nil, nil
		}
	})}}
	w := Workflow{File: workflow, PromptNode: "6", OutputNode: "9", OutputKey: "images"}
	path, err := c.Generate(context.Background(), w, "quoted \"prompt\" česky", out)
	if err != nil || path != out || requests != 3 {
		t.Fatalf("path=%s calls=%d err=%v", path, requests, err)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "PNG DATA" {
		t.Fatalf("download=%q err=%v", data, err)
	}
	if _, err := c.Generate(context.Background(), w, "overwrite", out); err == nil {
		t.Fatal("existing output overwritten")
	}
	if requests != 3 {
		t.Fatal("overwriting output should be rejected before queueing")
	}
}

func TestWorkflowFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "workflow.json")
			os.WriteFile(file, []byte(`{"1":{"class_type":"Text","inputs":{"text":""}},"2":{"class_type":"Save","inputs":{}}}`), 0600)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := Client{URL: "http://comfy.test", HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/prompt" {
					return reply(`{"prompt_id":"x"}`), nil
				}
				if mode == "cancel" {
					cancel()
					return reply(`{}`), nil
				}
				return reply(`{"x":{"status":{"status_str":"error","messages":["bad workflow"]}}}`), nil
			})}}
			out := filepath.Join(dir, "out.png")
			_, err := c.Generate(ctx, Workflow{File: file, PromptNode: "1", OutputNode: "2", OutputKey: "images"}, "test", out)
			if err == nil {
				t.Fatal("expected failure")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("failed job produced an output")
			}
		})
	}
}
