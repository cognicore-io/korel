package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cognicore/korel/pkg/korel/autotune/taxonomy"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

// Client calls an external LLM endpoint to approve/reject autotune suggestions.
type Client struct {
	Endpoint string
	APIKey   string

	HTTPClient *http.Client
	Prompts    PromptTemplates
}

// PromptTemplates allow customization of the LLM prompt text.
type PromptTemplates struct {
	Stopword string
	Taxonomy string
}

type requestPayload struct {
	Prompt string `json:"prompt"`
}

type responsePayload struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Approve implements stopwords.Reviewer.
func (c *Client) Approve(ctx context.Context, cand stoplist.Candidate) (bool, error) {
	prompt := c.stopwordPrompt(cand)
	resp, err := c.call(ctx, prompt)
	if err != nil {
		return false, err
	}
	return resp.Approve, nil
}

// ApproveTaxonomy implements taxonomy.Reviewer.
func (c *Client) ApproveTaxonomy(ctx context.Context, sugg taxonomy.Suggestion) (bool, error) {
	prompt := c.taxonomyPrompt(sugg)
	resp, err := c.call(ctx, prompt)
	if err != nil {
		return false, err
	}
	return resp.Approve, nil
}

func (c *Client) call(ctx context.Context, prompt string) (*responsePayload, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("llm reviewer: endpoint required")
	}

	body, err := json.Marshal(requestPayload{Prompt: prompt})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm reviewer: http %d", resp.StatusCode)
	}

	var payload responsePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) stopwordPrompt(cand stoplist.Candidate) string {
	tpl := c.Prompts.Stopword
	if tpl == "" {
		tpl = "Judge whether '%s' should be treated as a generic stopword. DF%%=%.1f, maxPMI=%.2f, entropy=%.2f. Reply with JSON {\"approve\": true|false}."
	}
	reason := cand.Reason
	return fmt.Sprintf(tpl, cand.Token, reason.IDF, reason.PMIMax, reason.CatEntropy)
}

func (c *Client) taxonomyPrompt(sugg taxonomy.Suggestion) string {
	tpl := c.Prompts.Taxonomy
	if tpl == "" {
		tpl = "Category '%s' is missing keyword '%s' in %d docs (confidence %.2f). Approve adding this keyword? Reply JSON {\"approve\": true|false}."
	}
	return fmt.Sprintf(tpl, sugg.Category, sugg.Keyword, sugg.MissedDocs, sugg.Confidence)
}
