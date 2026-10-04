package tools

import (
	"encoding/json"
	"fmt"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func schema(props string, required ...string) json.RawMessage {
	req, _ := json.Marshal(required)
	return json.RawMessage(fmt.Sprintf(`{"type":"object","properties":%s,"required":%s}`, props, req))
}

func Definitions() []backend.Tool {
	return []backend.Tool{
		{Type: "function", Function: backend.FunctionDef{
			Name:        "read",
			Description: "Read the contents of a file at the given path.",
			Parameters:  schema(`{"path":{"type":"string","description":"File path to read"}}`, "path"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "write",
			Description: "Create or overwrite a file with the given content. Creates parent directories as needed.",
			Parameters:  schema(`{"path":{"type":"string"},"content":{"type":"string"}}`, "path", "content"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "edit",
			Description: "Replace an exact, unique occurrence of old_string with new_string in a file.",
			Parameters:  schema(`{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}}`, "path", "old_string", "new_string"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "bash",
			Description: "Execute a shell command and return its combined stdout/stderr.",
			Parameters:  schema(`{"command":{"type":"string"}}`, "command"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "grep",
			Description: "Search for a regular expression pattern across files under a directory.",
			Parameters:  schema(`{"pattern":{"type":"string"},"path":{"type":"string","description":"Directory to search, defaults to current directory"},"max_matches":{"type":"integer","description":"Maximum number of matches to return, defaults to 200"}}`, "pattern"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "glob",
			Description: "Find files under a directory whose relative path matches a glob pattern (e.g. *.go, cmd/*/main.go).",
			Parameters:  schema(`{"pattern":{"type":"string"},"path":{"type":"string","description":"Directory to search, defaults to current directory"}}`, "pattern"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "web_search",
			Description: "Search the web for up-to-date information. Returns titles, URLs, and short snippets for the top results.",
			Parameters:  schema(`{"query":{"type":"string"},"max_results":{"type":"integer","description":"Defaults to 5"}}`, "query"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name:        "fetch_url",
			Description: "Fetch a URL and return its readable text content (HTML tags stripped). Use after web_search to read a specific page in full.",
			Parameters:  schema(`{"url":{"type":"string"}}`, "url"),
		}},
		{Type: "function", Function: backend.FunctionDef{
			Name: "browser_text",
			Description: "Render a local HTML file or URL in a real headless browser (JavaScript included) and return its visible text. " +
				"Use this over fetch_url when a page's content is generated/modified by JavaScript rather than present in the static HTML.",
			Parameters: schema(`{"path":{"type":"string","description":"Local file path or http(s) URL"}}`, "path"),
		}},
	}
}

func Dispatch(name string, argsJSON string) (string, error) {
	switch name {
	case "read":
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		return Read(a.Path)
	case "write":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		if a.Path == "" {
			return "", fmt.Errorf(`the "path" argument was missing or empty — arguments must include both "path" and "content" as top-level JSON fields, e.g. {"path":"notes.html","content":"..."}`)
		}
		return Write(a.Path, a.Content)
	case "edit":
		var a struct {
			Path      string `json:"path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		if a.Path == "" {
			return "", fmt.Errorf(`the "path" argument was missing or empty — arguments must include "path", "old_string", and "new_string" as top-level JSON fields`)
		}
		return Edit(a.Path, a.OldString, a.NewString)
	case "bash":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		return Bash(a.Command)
	case "grep":
		var a struct {
			Pattern    string `json:"pattern"`
			Path       string `json:"path"`
			MaxMatches int    `json:"max_matches"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		if a.Path == "" {
			a.Path = "."
		}
		return Grep(a.Pattern, a.Path, a.MaxMatches)
	case "glob":
		var a struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		if a.Path == "" {
			a.Path = "."
		}
		return Glob(a.Pattern, a.Path)
	case "web_search":
		var a struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		return WebSearch(a.Query, a.MaxResults)
	case "fetch_url":
		var a struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		return FetchURL(a.URL)
	case "browser_text":
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", err
		}
		return RenderedText(a.Path)
	default:
		return "", fmt.Errorf("unknown tool: %q (available tools: read, write, edit, bash, grep, glob, web_search, fetch_url, browser_text)", name)
	}
}
