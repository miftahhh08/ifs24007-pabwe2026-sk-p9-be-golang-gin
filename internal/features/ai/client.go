package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Completer adalah kontrak client OpenAI Chat Completions.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// OpenAIClient memanggil {OPENAI_ENDPOINT}/chat/completions.
type OpenAIClient struct {
	Endpoint string
	Key      string
	Model    string
	HTTP     *http.Client
}

// NewOpenAIClient membuat client dengan timeout 90 detik (generasi artikel bisa lama).
func NewOpenAIClient(endpoint, key, model string) *OpenAIClient {
	return &OpenAIClient{Endpoint: endpoint, Key: key, Model: model, HTTP: &http.Client{Timeout: 90 * time.Second}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete mengirim prompt dan mengembalikan teks jawaban pertama.
func (c *OpenAIClient) Complete(ctx context.Context, system, user string) (string, error) {
	// Marshal struct sederhana ini tidak mungkin gagal.
	body, _ := json.Marshal(chatRequest{
		Model:       c.Model,
		Temperature: 0.7,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	url := strings.TrimRight(c.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai returned status %d", resp.StatusCode)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("openai returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}
