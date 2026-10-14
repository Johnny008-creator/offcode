package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (*Message, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode response (status %d): %s", resp.StatusCode, string(raw))
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("backend error: %s", parsed.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(raw))
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("backend returned no choices: %s", string(raw))
	}

	return &parsed.Choices[0].Message, nil
}

// ContextLength best-effort queries an Ollama-native /api/show endpoint for
// the model's trained context length, falling back to LM Studio's REST
// metadata. Returns 0 if neither is available or the field can't be found.
func (c *Client) ContextLength(ctx context.Context, model string) int {
	if n := c.ollamaContextLength(ctx, model); n > 0 {
		return n
	}
	if m, ok := c.lmStudioModel(ctx, model); ok {
		// The loaded length is what the server will actually accept; the
		// trained maximum only applies once LM Studio reloads the model.
		if m.LoadedContextLength > 0 {
			return m.LoadedContextLength
		}
		return m.MaxContextLength
	}
	return 0
}

// lmStudioModelInfo is one entry of LM Studio's /api/v0/models listing.
type lmStudioModelInfo struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	State               string `json:"state"`
	MaxContextLength    int    `json:"max_context_length"`
	LoadedContextLength int    `json:"loaded_context_length"`
}

// lmStudioModels queries LM Studio's REST API, which, unlike the
// OpenAI-compatible /v1/models, reports model type and context lengths.
func (c *Client) lmStudioModels(ctx context.Context) ([]lmStudioModelInfo, bool) {
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/v0/models", nil)
	if err != nil {
		return nil, false
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var parsed struct {
		Data []lmStudioModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || len(parsed.Data) == 0 {
		return nil, false
	}
	return parsed.Data, true
}

func (c *Client) lmStudioModel(ctx context.Context, model string) (lmStudioModelInfo, bool) {
	models, _ := c.lmStudioModels(ctx)
	for _, m := range models {
		if m.ID == model {
			return m, true
		}
	}
	return lmStudioModelInfo{}, false
}

func (c *Client) ollamaContextLength(ctx context.Context, model string) int {
	root := strings.TrimSuffix(c.BaseURL, "/v1")

	body, _ := json.Marshal(map[string]string{"model": model})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/show", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var parsed struct {
		ModelInfo map[string]json.RawMessage `json:"model_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0
	}

	for key, raw := range parsed.ModelInfo {
		if !strings.HasSuffix(key, ".context_length") {
			continue
		}
		var n int
		if err := json.Unmarshal(raw, &n); err == nil {
			return n
		}
	}
	return 0
}

// ModelCapabilities returns Ollama's declared capability tags for a model
// (e.g. "completion", "vision", "tools", "thinking") via its native
// /api/show. Returns an empty slice (not an error) for backends without
// this endpoint (LM Studio) or any other failure — capability detection
// is a best-effort enhancement, not something that should block startup.
func (c *Client) ModelCapabilities(ctx context.Context, model string) []string {
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	body, _ := json.Marshal(map[string]string{"model": model})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/show", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var parsed struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil
	}
	return parsed.Capabilities
}

// SupportsVision is a convenience check over ModelCapabilities.
func (c *Client) SupportsVision(ctx context.Context, model string) bool {
	for _, cap := range c.ModelCapabilities(ctx, model) {
		if cap == "vision" {
			return true
		}
	}
	return false
}

// Embed returns one embedding vector per input string, via Ollama's
// native /api/embed (batched in a single request). Only meaningful
// against an embedding model (e.g. "nomic-embed-text") — used by
// internal/semsearch, not the chat completion path.
func (c *Client) Embed(ctx context.Context, model string, inputs []string) ([][]float32, error) {
	root := strings.TrimSuffix(strings.TrimRight(c.BaseURL, "/"), "/v1")
	body, err := json.Marshal(map[string]any{"model": model, "input": inputs})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", root, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("backend error: %s", parsed.Error)
	}
	if len(parsed.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(inputs), len(parsed.Embeddings))
	}
	for i, vector := range parsed.Embeddings {
		if len(vector) == 0 {
			return nil, fmt.Errorf("embedding %d is empty", i)
		}
		if len(vector) != len(parsed.Embeddings[0]) {
			return nil, fmt.Errorf("embedding %d has inconsistent dimensions", i)
		}
	}
	return parsed.Embeddings, nil
}

// ListModels returns the model names the backend currently has available.
// Ollama exposes its native /api/tags; other OpenAI-compatible backends
// (LM Studio, llama.cpp server) are queried via the standard GET /models.
func (c *Client) ListModels(ctx context.Context, backendKind string) ([]string, error) {
	if backendKind == "ollama" {
		return c.listModelsOllama(ctx)
	}
	return c.listModelsOpenAI(ctx)
}

func (c *Client) listModelsOllama(ctx context.Context) ([]string, error) {
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	names := make([]string, len(parsed.Models))
	for i, m := range parsed.Models {
		names[i] = m.Name
	}
	return names, nil
}

func (c *Client) listModelsOpenAI(ctx context.Context) ([]string, error) {
	// LM Studio's /v1/models also lists embedding models, which cannot
	// chat; its REST listing says which entries are chat models.
	if models, ok := c.lmStudioModels(ctx); ok {
		var names []string
		for _, m := range models {
			if m.Type == "llm" || m.Type == "vlm" {
				names = append(names, m.ID)
			}
		}
		return names, nil
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	names := make([]string, len(parsed.Data))
	for i, m := range parsed.Data {
		names[i] = m.ID
	}
	return names, nil
}

type LoadedModel struct {
	Name          string
	SizeBytes     uint64
	SizeVRAMBytes uint64
	ContextLength int
	ExpiresAt     string
}

// RunningModels queries Ollama's native /api/ps for models currently
// loaded in memory. Root is the Ollama server root (no /v1 suffix).
func (c *Client) RunningModels(ctx context.Context, root string) ([]LoadedModel, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/ps", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Models []struct {
			Name          string `json:"name"`
			Size          uint64 `json:"size"`
			SizeVRAM      uint64 `json:"size_vram"`
			ContextLength int    `json:"context_length"`
			ExpiresAt     string `json:"expires_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	out := make([]LoadedModel, len(parsed.Models))
	for i, m := range parsed.Models {
		out[i] = LoadedModel{
			Name:          m.Name,
			SizeBytes:     m.Size,
			SizeVRAMBytes: m.SizeVRAM,
			ContextLength: m.ContextLength,
			ExpiresAt:     m.ExpiresAt,
		}
	}
	return out, nil
}
