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
// the model's trained context length. Returns 0 if the backend doesn't
// support it (e.g. LM Studio) or the field can't be found.
func (c *Client) ContextLength(ctx context.Context, model string) int {
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
