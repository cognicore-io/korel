package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel"
)

type roundTrip func(*http.Request) *http.Response

func (rt roundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req), nil
}

func TestSummarizeSuccess(t *testing.T) {
	client := &Client{
		BaseURL: "https://api.test/v1/chat/completions",
		Model:   "gpt-test",
		HTTPClient: &http.Client{
			Transport: roundTrip(func(req *http.Request) *http.Response {
				body, _ := io.ReadAll(req.Body)
				if !strings.Contains(string(body), "Facts") {
					t.Fatalf("expected facts in payload")
				}
				return &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"role":"assistant","content":"Answer"}}]
					}`)),
					Header: make(http.Header),
				}
			}),
		},
	}

	out, err := client.Summarize(context.Background(), "question", []korel.Card{
		{
			Title:   "Card",
			Bullets: []string{"Fact"},
			Sources: []korel.SourceRef{{URL: "https://example.com", Time: time.Now()}},
		},
	})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if out != "Answer" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestSummarizeError(t *testing.T) {
	client := &Client{
		BaseURL: "https://api.test/v1/chat/completions",
		Model:   "gpt-test",
		HTTPClient: &http.Client{
			Transport: roundTrip(func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad"}}`)),
					Header:     make(http.Header),
				}
			}),
		},
	}
	if _, err := client.Summarize(context.Background(), "q", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestChat(t *testing.T) {
	client := &Client{
		BaseURL: "https://api.test/v1/chat/completions",
		Model:   "gpt-test",
		HTTPClient: &http.Client{
			Transport: roundTrip(func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`)),
					Header:     make(http.Header),
				}
			}),
		},
	}
	out, err := client.Chat(context.Background(), "system", "user prompt")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if out != "hi" {
		t.Fatalf("unexpected chat output %s", out)
	}
}
