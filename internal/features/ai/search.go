package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSources = 5

// Searcher adalah kontrak client Mesin Pencari.
type Searcher interface {
	Search(ctx context.Context, query string) ([]Source, error)
}

// SearchClient memanggil SEARCH_ENGINE_ENDPOINT (GET ?q=...).
type SearchClient struct {
	Endpoint string
	HTTP     *http.Client
}

// NewSearchClient membuat client dengan timeout 15 detik.
func NewSearchClient(endpoint string) *SearchClient {
	return &SearchClient{Endpoint: endpoint, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

type rawItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

type rawSearch struct {
	Results []rawItem `json:"results"`
	Data    []rawItem `json:"data"`
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Search mengambil hasil penelusuran; format respons toleran ("results" atau "data").
func (c *SearchClient) Search(ctx context.Context, query string) ([]Source, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	qs.Set("q", query)
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search engine returned status %d", resp.StatusCode)
	}
	var raw rawSearch
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	items := raw.Results
	if len(items) == 0 {
		items = raw.Data
	}
	sources := make([]Source, 0, maxSources)
	for _, it := range items {
		if len(sources) == maxSources {
			break
		}
		link := first(it.URL, it.Link)
		if link == "" {
			continue
		}
		sources = append(sources, Source{
			Title:   first(it.Title, link),
			URL:     link,
			Snippet: first(it.Snippet, it.Content, it.Description),
		})
	}
	return sources, nil
}
