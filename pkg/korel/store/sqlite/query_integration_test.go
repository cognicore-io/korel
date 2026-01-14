package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// TestQueryRetrieverIntegration tests document retrieval operations with real SQLite
// This verifies GetDocsByTokens and TopNeighbors work correctly for query workflows
func TestQueryRetrieverIntegration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()

	// Insert test documents
	docs := []store.Doc{
		{
			URL:         "https://example.com/ml",
			Title:       "Machine Learning Basics",
			PublishedAt: time.Now().Add(-2 * time.Hour),
			Tokens:      []string{"machine", "learning", "basics"},
			Cats:        []string{"ai"},
		},
		{
			URL:         "https://example.com/nn",
			Title:       "Neural Networks",
			PublishedAt: time.Now().Add(-1 * time.Hour),
			Tokens:      []string{"neural", "network", "deep"},
			Cats:        []string{"ai"},
		},
		{
			URL:         "https://example.com/dl",
			Title:       "Deep Learning Guide",
			PublishedAt: time.Now(),
			Tokens:      []string{"deep", "learning", "guide"},
			Cats:        []string{"ai"},
		},
	}

	for _, doc := range docs {
		if err := st.UpsertDoc(ctx, doc); err != nil {
			t.Fatalf("UpsertDoc %s: %v", doc.URL, err)
		}
	}

	// Set up token DFs
	st.UpsertTokenDF(ctx, "machine", 10)
	st.UpsertTokenDF(ctx, "learning", 15)
	st.UpsertTokenDF(ctx, "deep", 8)
	st.UpsertTokenDF(ctx, "neural", 6)

	// Set up PMI pairs
	st.IncPair(ctx, "machine", "learning")
	st.IncPair(ctx, "learning", "deep")

	// Test 1: Query for "machine" - should find ML doc
	results, err := st.GetDocsByTokens(ctx, []string{"machine"}, 10)
	if err != nil {
		t.Fatalf("GetDocsByTokens(machine): %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 doc for 'machine', got %d", len(results))
	}

	if len(results) > 0 && results[0].Title != "Machine Learning Basics" {
		t.Errorf("Expected ML doc, got %q", results[0].Title)
	}

	// Test 2: Query for "learning" - should find ML and DL docs
	results, err = st.GetDocsByTokens(ctx, []string{"learning"}, 10)
	if err != nil {
		t.Fatalf("GetDocsByTokens(learning): %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 docs for 'learning', got %d", len(results))
	}

	// Verify recency ordering (newest first)
	if len(results) == 2 {
		if results[0].PublishedAt.Before(results[1].PublishedAt) {
			t.Error("Results should be ordered by recency (newest first)")
		}
	}

	// Test 3: PMI neighbors for "machine" - should include "learning"
	neighbors, err := st.TopNeighbors(ctx, "machine", 5)
	if err != nil {
		t.Fatalf("TopNeighbors(machine): %v", err)
	}

	foundLearning := false
	for _, n := range neighbors {
		if n.Token == "learning" {
			foundLearning = true
			break
		}
	}

	if !foundLearning {
		t.Error("Expected 'learning' to be a neighbor of 'machine'")
	}

	// Test 4: Multi-token query
	results, err = st.GetDocsByTokens(ctx, []string{"machine", "neural"}, 10)
	if err != nil {
		t.Fatalf("GetDocsByTokens(machine, neural): %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 docs for 'machine' OR 'neural', got %d", len(results))
	}

	// Test 5: Empty query
	results, err = st.GetDocsByTokens(ctx, []string{}, 10)
	if err != nil {
		t.Fatalf("GetDocsByTokens(empty): %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 docs for empty query, got %d", len(results))
	}

	// Test 6: Non-existent token
	results, err = st.GetDocsByTokens(ctx, []string{"nonexistent"}, 10)
	if err != nil {
		t.Fatalf("GetDocsByTokens(nonexistent): %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 docs for non-existent token, got %d", len(results))
	}

	// Test 7: Limit enforcement
	results, err = st.GetDocsByTokens(ctx, []string{"learning"}, 1)
	if err != nil {
		t.Fatalf("GetDocsByTokens with limit=1: %v", err)
	}

	if len(results) > 1 {
		t.Errorf("Limit=1 should return at most 1 doc, got %d", len(results))
	}

	t.Logf("Query retrieval integration tests passed")
}
