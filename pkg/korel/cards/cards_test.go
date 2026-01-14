package cards

import (
	"strings"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/rank"
)

func TestBuilderEmptyDocs(t *testing.T) {
	builder := New()
	query := rank.Query{
		Tokens:     []string{"test"},
		Categories: []string{"ai"},
	}

	card := builder.Build("Test Card", []ScoredDoc{}, query, nil)

	if len(card.Bullets) != 0 {
		t.Errorf("Empty docs should produce 0 bullets, got %d", len(card.Bullets))
	}

	if len(card.Sources) != 0 {
		t.Errorf("Empty docs should produce 0 sources, got %d", len(card.Sources))
	}

	// Scores should be 0 (0/0 = NaN, but we check n>0)
	if card.ScoreBreakdown["pmi"] != 0 {
		t.Errorf("Empty docs should have 0 PMI score")
	}
}

func TestBuilderULIDUniqueness(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{"test"}}
	docs := []ScoredDoc{
		{Title: "Doc 1", URL: "http://example.com/1", Time: time.Now()},
	}

	// Generate multiple cards rapidly
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		card := builder.Build("Card", docs, query, nil)
		if ids[card.ID] {
			t.Errorf("Duplicate ULID generated: %s", card.ID)
		}
		ids[card.ID] = true
	}

	if len(ids) != 1000 {
		t.Errorf("Expected 1000 unique IDs, got %d", len(ids))
	}
}

func TestBuilderMatchedTokensOnlyQuery(t *testing.T) {
	builder := New()
	query := rank.Query{
		Tokens:     []string{"machine", "learning"},
		Categories: []string{},
	}

	docs := []ScoredDoc{
		{
			Tokens: []string{"machine", "learning", "neural", "network", "deep"},
			Breakdown: rank.ScoreBreakdown{
				PMI: 1.5,
			},
		},
		{
			Tokens: []string{"learning", "algorithm", "optimization"},
			Breakdown: rank.ScoreBreakdown{
				PMI: 1.2,
			},
		},
	}

	card := builder.Build("Test", docs, query, nil)

	// Should only have "machine" and "learning" (query tokens that appear in docs)
	if len(card.Explain.MatchedTokens) > 2 {
		t.Errorf("Should only match query tokens, got %d: %v", len(card.Explain.MatchedTokens), card.Explain.MatchedTokens)
	}

	hasNeural := false
	for _, tok := range card.Explain.MatchedTokens {
		if tok == "neural" || tok == "network" || tok == "deep" || tok == "algorithm" || tok == "optimization" {
			hasNeural = true
		}
	}

	if hasNeural {
		t.Errorf("Should not include non-query tokens in MatchedTokens: %v", card.Explain.MatchedTokens)
	}
}

func TestBuilderScoreAggregation(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{"test"}}

	docs := []ScoredDoc{
		{
			Title:  "Doc 1",
			URL:    "http://example.com/1",
			Time:   time.Now(),
			Tokens: []string{"test"},
			Breakdown: rank.ScoreBreakdown{
				PMI:       2.0,
				Cats:      0.8,
				Recency:   0.9,
				Authority: 0.5,
				Len:       0.1,
			},
		},
		{
			Title:  "Doc 2",
			URL:    "http://example.com/2",
			Time:   time.Now(),
			Tokens: []string{"test"},
			Breakdown: rank.ScoreBreakdown{
				PMI:       1.0,
				Cats:      0.6,
				Recency:   0.7,
				Authority: 0.3,
				Len:       0.2,
			},
		},
	}

	card := builder.Build("Test", docs, query, nil)

	// Check averages
	expectedPMI := (2.0 + 1.0) / 2.0
	if card.ScoreBreakdown["pmi"] != expectedPMI {
		t.Errorf("PMI average should be %f, got %f", expectedPMI, card.ScoreBreakdown["pmi"])
	}

	expectedCats := (0.8 + 0.6) / 2.0
	if card.ScoreBreakdown["cats"] != expectedCats {
		t.Errorf("Cats average should be %f, got %f", expectedCats, card.ScoreBreakdown["cats"])
	}
}

func TestBuilderCategoryOverlap(t *testing.T) {
	builder := New()
	query := rank.Query{
		Tokens:     []string{},
		Categories: []string{"ai", "ml"},
	}

	docs := []ScoredDoc{
		{Title: "Test", URL: "http://test.com", Time: time.Now()},
	}

	card := builder.Build("Test", docs, query, nil)

	if len(card.Explain.CategoryOverlap) != 2 {
		t.Errorf("Should preserve query categories, got %d", len(card.Explain.CategoryOverlap))
	}
}

func TestBuilderQueryTokens(t *testing.T) {
	builder := New()
	queryTokens := []string{"neural", "network", "architecture"}
	query := rank.Query{
		Tokens:     queryTokens,
		Categories: []string{},
	}

	docs := []ScoredDoc{
		{Title: "Test", URL: "http://test.com", Time: time.Now(), Tokens: []string{"neural"}},
	}

	card := builder.Build("Test", docs, query, nil)

	if len(card.Explain.QueryTokens) != 3 {
		t.Errorf("Should preserve all query tokens, got %d", len(card.Explain.QueryTokens))
	}

	for i, tok := range card.Explain.QueryTokens {
		if tok != queryTokens[i] {
			t.Errorf("Query token %d should be %q, got %q", i, queryTokens[i], tok)
		}
	}
}

func TestBuilderTopPairs(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{}}
	docs := []ScoredDoc{
		{Title: "Test", URL: "http://test.com", Time: time.Now()},
	}

	topPairs := [][3]interface{}{
		{"token1", "token2", 2.5},
		{"token3", "token4", 1.8},
	}

	card := builder.Build("Test", docs, query, topPairs)

	if len(card.Explain.TopPairs) != 2 {
		t.Errorf("Should preserve top pairs, got %d", len(card.Explain.TopPairs))
	}

	if card.Explain.TopPairs[0][2] != 2.5 {
		t.Errorf("First pair PMI should be 2.5, got %v", card.Explain.TopPairs[0][2])
	}
}

func TestBuilderMultipleSources(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{}}

	time1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	docs := []ScoredDoc{
		{Title: "Doc 1", URL: "http://example.com/1", Time: time1},
		{Title: "Doc 2", URL: "http://example.com/2", Time: time2},
	}

	card := builder.Build("Test", docs, query, nil)

	if len(card.Sources) != 2 {
		t.Errorf("Should have 2 sources, got %d", len(card.Sources))
	}

	if card.Sources[0].URL != "http://example.com/1" {
		t.Errorf("First source URL incorrect: %s", card.Sources[0].URL)
	}

	if !card.Sources[0].Time.Equal(time1) {
		t.Errorf("First source time incorrect: %v", card.Sources[0].Time)
	}
}

func TestBuilderBullets(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{}}

	docs := []ScoredDoc{
		{Title: "Machine learning advances", URL: "http://example.com/1", Time: time.Now()},
		{Title: "Neural network architecture", URL: "http://example.com/2", Time: time.Now()},
	}

	card := builder.Build("Test", docs, query, nil)

	if len(card.Bullets) != 2 {
		t.Errorf("Should have 2 bullets, got %d", len(card.Bullets))
	}

	if card.Bullets[0] != "Machine learning advances" {
		t.Errorf("First bullet incorrect: %s", card.Bullets[0])
	}

	if card.Bullets[1] != "Neural network architecture" {
		t.Errorf("Second bullet incorrect: %s", card.Bullets[1])
	}
}

func TestBuilderTitle(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{}}
	docs := []ScoredDoc{
		{Title: "Test", URL: "http://test.com", Time: time.Now()},
	}

	title := "Machine Learning Overview"
	card := builder.Build(title, docs, query, nil)

	if card.Title != title {
		t.Errorf("Title should be %q, got %q", title, card.Title)
	}
}

func TestBuilderULIDFormat(t *testing.T) {
	builder := New()
	query := rank.Query{Tokens: []string{}}
	docs := []ScoredDoc{
		{Title: "Test", URL: "http://test.com", Time: time.Now()},
	}

	card := builder.Build("Test", docs, query, nil)

	// ULID should be 26 characters (base32 encoded)
	if len(card.ID) != 26 {
		t.Errorf("ULID should be 26 characters, got %d: %s", len(card.ID), card.ID)
	}

	// ULID should only contain valid base32 characters
	validChars := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for _, c := range card.ID {
		if !strings.ContainsRune(validChars, c) {
			t.Errorf("Invalid ULID character: %c in %s", c, card.ID)
		}
	}
}

func TestBuilderNoQueryTokenMatch(t *testing.T) {
	builder := New()
	query := rank.Query{
		Tokens: []string{"machine", "learning"},
	}

	docs := []ScoredDoc{
		{
			Title:  "Document about biology",
			URL:    "http://example.com",
			Time:   time.Now(),
			Tokens: []string{"biology", "cell", "organism"},
		},
	}

	card := builder.Build("Test", docs, query, nil)

	// No query tokens appear in doc, so MatchedTokens should be empty
	if len(card.Explain.MatchedTokens) != 0 {
		t.Errorf("No query tokens match, should have 0 matched tokens, got %d: %v",
			len(card.Explain.MatchedTokens), card.Explain.MatchedTokens)
	}
}
