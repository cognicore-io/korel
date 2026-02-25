package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/rank"
)

func TestParserBasic(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{"the", "a", "and"})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
	})
	taxonomy := ingest.NewTaxonomy()
	taxonomy.AddSector("ai", []string{"machine learning"})

	qp := NewParser(tokenizer, parser, taxonomy)

	queryStr := "machine learning models"
	query := qp.Parse(queryStr)

	// Should have tokens
	if len(query.Tokens) == 0 {
		t.Error("Query should have tokens")
	}

	// Should have recognized "machine learning"
	found := false
	for _, tok := range query.Tokens {
		if tok == "machine learning" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should recognize 'machine learning', got %v", query.Tokens)
	}

	// Should have ai category
	if len(query.Categories) != 1 || query.Categories[0] != "ai" {
		t.Errorf("Expected [ai] category, got %v", query.Categories)
	}
}

func TestParserEmptyQuery(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("")

	if len(query.Tokens) != 0 {
		t.Errorf("Empty query should have 0 tokens, got %d", len(query.Tokens))
	}

	if len(query.Categories) != 0 {
		t.Errorf("Empty query should have 0 categories, got %d", len(query.Categories))
	}
}

func TestParserStopwordsOnly(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{"the", "a", "and", "of", "in"})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("the and the of in")

	if len(query.Tokens) != 0 {
		t.Errorf("Stopwords-only query should have 0 tokens, got %d: %v", len(query.Tokens), query.Tokens)
	}
}

func TestParserVariantExpansion(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
	})
	taxonomy := ingest.NewTaxonomy()
	taxonomy.AddSector("ai", []string{"machine learning"})

	qp := NewParser(tokenizer, parser, taxonomy)

	// Query with variant "ml"
	query := qp.Parse("ml models")

	// Should expand "ml" to "machine learning"
	found := false
	for _, tok := range query.Tokens {
		if tok == "machine learning" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should expand 'ml' to 'machine learning', got %v", query.Tokens)
	}

	// Should have ai category
	if len(query.Categories) != 1 || query.Categories[0] != "ai" {
		t.Errorf("Expected [ai] category, got %v", query.Categories)
	}
}

func TestParserMultipleCategories(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()
	taxonomy.AddSector("ai", []string{"machine-learning"})
	taxonomy.AddSector("nlp", []string{"natural-language"})
	taxonomy.AddEvent("release", []string{"released", "launched"})

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("machine-learning natural-language model released")

	// Should have multiple categories
	if len(query.Categories) != 3 {
		t.Errorf("Expected 3 categories, got %d: %v", len(query.Categories), query.Categories)
	}

	expected := map[string]bool{"ai": true, "nlp": true, "release": true}
	for _, cat := range query.Categories {
		if !expected[cat] {
			t.Errorf("Unexpected category: %s", cat)
		}
	}
}

func TestParserNoMatchingCategories(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()
	taxonomy.AddSector("finance", []string{"stock", "bond"})

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("machine learning neural network")

	// Should have tokens but no categories
	if len(query.Tokens) == 0 {
		t.Error("Should have tokens")
	}

	if len(query.Categories) != 0 {
		t.Errorf("Expected 0 categories for unrelated query, got %d: %v", len(query.Categories), query.Categories)
	}
}

func TestParserVeryLongQuery(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	// Very long query with 100+ words
	longQuery := ""
	for i := 0; i < 100; i++ {
		longQuery += "word "
	}

	query := qp.Parse(longQuery)

	// Should handle long queries
	if len(query.Tokens) == 0 {
		t.Error("Long query should produce tokens")
	}
}

func TestParserSpecialCharacters(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("hello@world.com test#tag C++ python3.9")

	// Should filter out special characters (letters, digits, and hyphens are allowed)
	for _, tok := range query.Tokens {
		for _, r := range tok {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Errorf("Token %s contains special characters", tok)
			}
		}
	}
}

func TestParserCaseNormalization(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	// Mixed case query
	query := qp.Parse("MACHINE Learning NEURAL Network")

	// All tokens should be lowercased
	for _, tok := range query.Tokens {
		for _, r := range tok {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("Token %s contains uppercase characters", tok)
			}
		}
	}
}

func TestParserMultiTokenRecognition(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{
		{Canonical: "neural network", Variants: []string{}, Category: "ai"},
		{Canonical: "machine learning", Variants: []string{}, Category: "ai"},
	})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("neural network and machine learning")

	// Should recognize both multi-token phrases
	hasNN := false
	hasML := false
	for _, tok := range query.Tokens {
		if tok == "neural network" {
			hasNN = true
		}
		if tok == "machine learning" {
			hasML = true
		}
	}

	if !hasNN || !hasML {
		t.Errorf("Should recognize multi-token phrases, got %v", query.Tokens)
	}
}

// Edge case tests

func TestParserWhitespaceOnly(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("   \t\n\r   ")

	if len(query.Tokens) != 0 {
		t.Errorf("Whitespace-only query should have 0 tokens, got %d", len(query.Tokens))
	}
}

func TestParserPunctuationOnly(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("!@#$%^&*()_+-=[]{}|;':\",./<>?")

	if len(query.Tokens) != 0 {
		t.Errorf("Punctuation-only query should have 0 tokens, got %d: %v", len(query.Tokens), query.Tokens)
	}
}

func TestParserNumbersOnly(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("123 456 789 2023")

	// Numbers should be filtered out
	if len(query.Tokens) != 0 {
		t.Errorf("Numbers-only query should have 0 tokens, got %d: %v", len(query.Tokens), query.Tokens)
	}
}

func TestParserRepeatedWords(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("test test test machine machine learning learning")

	// Should preserve repeated words (not deduplicate)
	// The parser doesn't deduplicate, tokenizer produces all tokens
	if len(query.Tokens) == 0 {
		t.Error("Should have tokens")
	}
}

func TestParserEmptyComponents(t *testing.T) {
	// All components are empty/minimal
	tokenizer := ingest.NewTokenizer([]string{})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("simple test query")

	// Should still work with minimal components
	if len(query.Tokens) == 0 {
		t.Error("Should have tokens even with empty components")
	}

	if len(query.Categories) != 0 {
		t.Error("Empty taxonomy should produce no categories")
	}
}

func TestParserNilHandling(t *testing.T) {
	// Test that parser handles nil gracefully (should not panic)
	tokenizer := ingest.NewTokenizer(nil)
	parser := ingest.NewMultiTokenParser(nil)
	taxonomy := ingest.NewTaxonomy()

	qp := NewParser(tokenizer, parser, taxonomy)

	query := qp.Parse("test query")

	// Should not panic
	_ = query
}

func TestRetrieverCreation(t *testing.T) {
	// Test that retriever can be created
	mockStore := &mockStore{}
	retriever := NewRetriever(mockStore)

	if retriever == nil {
		t.Error("NewRetriever should return non-nil retriever")
	}
}

func TestRetrieverRetrieve(t *testing.T) {
	mockStore := &mockStore{
		docs: []StoreDoc{
			{
				ID:          1,
				URL:         "https://example.com/1",
				Title:       "Machine Learning Article",
				PublishedAt: time.Now(),
				Cats:        []string{"ai"},
				LinksOut:    5,
				Tokens:      []string{"machine", "learning"},
			},
			{
				ID:          2,
				URL:         "https://example.com/2",
				Title:       "Neural Networks",
				PublishedAt: time.Now(),
				Cats:        []string{"ai"},
				LinksOut:    3,
				Tokens:      []string{"neural", "network"},
			},
		},
		neighbors: []Neighbor{
			{Token: "neural", PMI: 2.5},
			{Token: "deep", PMI: 2.0},
		},
	}

	retriever := NewRetriever(mockStore)
	ctx := context.Background()

	query := rank.Query{
		Tokens:     []string{"machine"},
		Categories: []string{"ai"},
	}

	candidates, err := retriever.Retrieve(ctx, query, 10)

	if err != nil {
		t.Fatalf("Retrieve should succeed, got error: %v", err)
	}

	if len(candidates) == 0 {
		t.Error("Should retrieve at least one candidate")
	}

	// Should deduplicate
	seen := make(map[int64]bool)
	for _, c := range candidates {
		if seen[c.DocID] {
			t.Error("Should not have duplicate candidates")
		}
		seen[c.DocID] = true
	}
}

func TestRetrieverNilStore(t *testing.T) {
	retriever := NewRetriever(nil)
	ctx := context.Background()

	query := rank.Query{
		Tokens: []string{"test"},
	}

	_, err := retriever.Retrieve(ctx, query, 10)

	if err == nil {
		t.Error("Should fail with nil store")
	}
}

func TestRetrieverEmptyQuery(t *testing.T) {
	mockStore := &mockStore{docs: []StoreDoc{}}
	retriever := NewRetriever(mockStore)
	ctx := context.Background()

	query := rank.Query{
		Tokens:     []string{},
		Categories: []string{},
	}

	candidates, err := retriever.Retrieve(ctx, query, 10)

	if err != nil {
		t.Errorf("Empty query should not error, got: %v", err)
	}

	// Empty query may return empty results
	_ = candidates
}

func TestRetrieverZeroLimit(t *testing.T) {
	mockStore := &mockStore{
		docs: []StoreDoc{
			{ID: 1, Tokens: []string{"test"}},
		},
	}
	retriever := NewRetriever(mockStore)
	ctx := context.Background()

	query := rank.Query{Tokens: []string{"test"}}

	candidates, err := retriever.Retrieve(ctx, query, 0)

	if err != nil {
		t.Errorf("Zero limit should not error, got: %v", err)
	}

	// Should use default limit
	_ = candidates
}

func TestRetrieverStoreError(t *testing.T) {
	mockStore := &mockStore{
		shouldError: true,
	}
	retriever := NewRetriever(mockStore)
	ctx := context.Background()

	query := rank.Query{Tokens: []string{"test"}}

	_, err := retriever.Retrieve(ctx, query, 10)

	if err == nil {
		t.Error("Should propagate store error")
	}
}

// Mock store for testing
type mockStore struct {
	docs        []StoreDoc
	neighbors   []Neighbor
	shouldError bool
}

func (m *mockStore) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]StoreDoc, error) {
	if m.shouldError {
		return nil, errors.New("mock store error")
	}
	return m.docs, nil
}

func (m *mockStore) TopNeighbors(ctx context.Context, token string, k int) ([]Neighbor, error) {
	if m.shouldError {
		return nil, errors.New("mock store error")
	}
	return m.neighbors, nil
}
