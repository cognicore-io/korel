package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cognicore/korel/pkg/korel"
)

// Client calls an OpenAI-compatible chat completion endpoint.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string

	HTTPClient *http.Client
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Summarize builds a grounded answer from Korel cards.
func (c *Client) Summarize(ctx context.Context, query string, cards []korel.Card) (string, error) {
	system := "You are a grounded analyst. Answer using ONLY the provided facts. Cite sources."
	user := formatPrompt(query, cards)
	return c.Chat(ctx, system, user)
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if c.BaseURL == "" || c.Model == "" {
		return "", fmt.Errorf("llm: base URL and model required")
	}
	messages := []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
	payload, err := c.send(ctx, messages)
	if err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("llm: empty response")
	}
	return payload.Choices[0].Message.Content, nil
}

func (c *Client) send(ctx context.Context, messages []chatMessage) (*chatResponse, error) {
	reqBody, err := json.Marshal(chatRequest{Model: c.Model, Messages: messages})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Error != nil {
		return nil, fmt.Errorf("llm error: %s", payload.Error.Message)
	}
	return &payload, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func formatPrompt(query string, cards []korel.Card) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Question: %s\nFacts:\n", query)
	for idx, card := range cards {
		fmt.Fprintf(&buf, "%d. %s\n", idx+1, card.Title)
		for _, bullet := range card.Bullets {
			fmt.Fprintf(&buf, "   - %s\n", bullet)
		}
		for _, src := range card.Sources {
			fmt.Fprintf(&buf, "   Source: %s (%s)\n", src.URL, src.Time.Format("2006-01-02"))
		}
	}
	fmt.Fprintf(&buf, "\nRespond with a concise answer using these facts and cite sources explicitly.\n")
	return buf.String()
}
