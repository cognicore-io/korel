package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cognicore/korel/pkg/korel/autotune/taxonomy"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

type stubTransport struct {
	fn func(*http.Request) *http.Response
}

func (s stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.fn(req), nil
}

func TestClientApproveStopword(t *testing.T) {
	client := &Client{
		Endpoint: "https://llm.local/stopword",
		HTTPClient: &http.Client{
			Transport: stubTransport{
				fn: func(req *http.Request) *http.Response {
					body, _ := io.ReadAll(req.Body)
					if !strings.Contains(string(body), "stopword") {
						t.Fatalf("expected prompt payload, got %q", body)
					}
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(`{"approve": true}`)),
						Header:     make(http.Header),
					}
				},
			},
		},
	}
	ok, err := client.Approve(context.Background(), stoplist.Candidate{
		Token: "the",
		Reason: stoplist.Reason{
			IDF:        0.1,
			PMIMax:     0.02,
			CatEntropy: 0.9,
		},
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !ok {
		t.Fatal("expected approval")
	}
}

func TestClientApproveTaxonomy(t *testing.T) {
	client := &Client{
		Endpoint: "https://llm.local/taxonomy",
		HTTPClient: &http.Client{
			Transport: stubTransport{
				fn: func(req *http.Request) *http.Response {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(`{"approve": false}`)),
						Header:     make(http.Header),
					}
				},
			},
		},
	}
	ok, err := client.ApproveTaxonomy(context.Background(), taxonomy.Suggestion{
		Category:   "ai",
		Keyword:    "transformer",
		MissedDocs: 25,
		Confidence: 0.8,
	})
	if err != nil {
		t.Fatalf("ApproveTaxonomy: %v", err)
	}
	if ok {
		t.Fatal("expected rejection")
	}
}

func TestClientHTTPError(t *testing.T) {
	client := &Client{Endpoint: "http://127.0.0.1:0"} // invalid
	if _, err := client.Approve(context.Background(), stoplist.Candidate{}); err == nil {
		t.Fatal("expected error")
	}
}
