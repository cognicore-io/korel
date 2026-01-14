package main

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel"
)

func TestChatCLIIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "korel.db")

	root := repoRoot(t)
	stoplist := filepath.Join(root, "testdata", "hn", "stoplist.yaml")
	dict := filepath.Join(root, "testdata", "hn", "tokens.dict")
	taxonomy := filepath.Join(root, "testdata", "hn", "taxonomies.yaml")
	fixture := filepath.Join(root, "testdata", "integration", "docs.jsonl")

	engine, cleanup, err := buildEngine(
		ctx,
		dbPath,
		stoplist,
		dict,
		taxonomy,
		"",
	)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	defer cleanup()

	items, err := rss.LoadFromJSONL(fixture)
	if err != nil {
		t.Fatalf("load docs: %v", err)
	}

	for _, item := range items {
		doc := korel.IngestDoc{
			URL:         item.URL,
			Title:       item.Title,
			Outlet:      item.Outlet,
			PublishedAt: item.PublishedAt,
			BodyText:    item.Body,
			SourceCats:  item.SourceCats,
		}
		if err := engine.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingest %s: %v", item.URL, err)
		}
	}

	res, err := engine.Search(ctx, korel.SearchRequest{
		Query: "solar policy",
		TopK:  2,
		Now:   time.Now(),
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Cards) == 0 {
		t.Fatalf("expected at least one card, got 0")
	}

	card := res.Cards[0]
	if len(card.Bullets) == 0 {
		t.Fatalf("card should include bullets: %+v", card)
	}
	if len(card.Explain.QueryTokens) == 0 {
		t.Fatalf("card should include explain query tokens")
	}
	if len(card.Explain.ExpandedTokens) == 0 {
		t.Fatalf("expected expanded tokens from inference layer")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
