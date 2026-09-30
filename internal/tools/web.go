package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// searxngURL points at a local SearXNG instance offcode expects to be
// running (docker container "offcode-searxng", set up once and left
// running with --restart unless-stopped — see docs/DECISIONS.md).
const searxngURL = "http://127.0.0.1:8888/search"

var webHTTPClient = &http.Client{Timeout: 20 * time.Second}

type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

func WebSearch(query string, maxResults int) (string, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	req, err := http.NewRequest(http.MethodGet, searxngURL, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Set("q", query)
	q.Set("format", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := webHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web search returned status %d", resp.StatusCode)
	}

	var parsed searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode search response: %w", err)
	}

	if len(parsed.Results) == 0 {
		return "no results", nil
	}
	if len(parsed.Results) > maxResults {
		parsed.Results = parsed.Results[:maxResults]
	}

	var b strings.Builder
	for i, r := range parsed.Results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, r.Title, r.URL)
		if r.Content != "" {
			fmt.Fprintf(&b, "   %s\n", r.Content)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

var (
	reScript = regexp.MustCompile(`(?is)<script.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style.*?</style>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]+>`)
	reBlank  = regexp.MustCompile(`\n{3,}`)
)

const maxFetchChars = 8000

func FetchURL(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "offcode/"+"0.1.0")

	resp, err := webHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}

	text := string(body)
	if strings.Contains(resp.Header.Get("Content-Type"), "html") {
		text = reScript.ReplaceAllString(text, "")
		text = reStyle.ReplaceAllString(text, "")
		text = reTag.ReplaceAllString(text, "\n")
		text = reBlank.ReplaceAllString(text, "\n\n")
		text = strings.TrimSpace(text)
	}

	if len(text) > maxFetchChars {
		text = text[:maxFetchChars] + fmt.Sprintf("\n… (truncated, %d more chars)", len(text)-maxFetchChars)
	}
	return text, nil
}
