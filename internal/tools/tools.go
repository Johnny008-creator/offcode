package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

func schema(props string, required ...string) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	req, _ := json.Marshal(required)
	return json.RawMessage(fmt.Sprintf(`{"type":"object","properties":%s,"required":%s}`, props, req))
}

func rawKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func Definitions() []backend.Tool {
	return append([]backend.Tool{
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
	}, extensionDefinitions()...)
}

func Dispatch(name string, argsJSON string) (string, error) {
	return DispatchContext(context.Background(), name, argsJSON)
}

func DispatchContext(ctx context.Context, name string, argsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
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
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
			return "", err
		}
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
		// A missing "content" key alongside some other, unexpected key is a
		// corrupted tool call (observed live: the chat template's own
		// tag-extraction mangled the parameter name into an empty string,
		// silently writing an empty file otherwise) — not a deliberate
		// empty file, which would just omit "content" entirely.
		if _, hasContent := raw["content"]; !hasContent && len(raw) > 1 {
			return "", fmt.Errorf(`the "content" argument is missing (got keys %v instead) — retry with both "path" and "content" as top-level JSON fields, e.g. {"path":"notes.html","content":"..."}`, rawKeys(raw))
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
		return BashContext(ctx, a.Command)
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
		return dispatchExtension(ctx, name, argsJSON)
	}
}
