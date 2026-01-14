package korel

import (
	"context"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/cards"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/query"
	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/cognicore/korel/pkg/korel/store"
)

// TestEndToEnd demonstrates the complete Korel workflow:
// 1. Configuration loading
// 2. Document ingestion
// 3. Query parsing
// 4. Candidate retrieval
// 5. Ranking
// 6. Card generation
func TestEndToEnd(t *testing.T) {
	ctx := context.Background()

	// === Phase 1: Setup Configuration ===

	// Create in-memory components (no file loading needed for test)
	tokenizer := ingest.NewTokenizer([]string{"the", "a", "and", "of", "in"})

	mtParser := ingest.NewMultiTokenParser([]ingest.DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
		{Canonical: "neural network", Variants: []string{"nn"}, Category: "ai"},
		{Canonical: "deep learning", Variants: []string{"dl"}, Category: "ai"},
	})

	taxonomy := ingest.NewTaxonomy()
	taxonomy.AddSector("ai", []string{"machine learning", "neural network", "deep learning"})
	taxonomy.AddSector("finance", []string{"stock", "trading", "investment"})
	taxonomy.AddEvent("release", []string{"released", "launched", "announced"})

	// Create pipeline
	pipeline := ingest.NewPipeline(tokenizer, mtParser, taxonomy)

	// Create inference engine
	inf := simple.New()
	inf.LoadRules("is_a(bert, transformer).\nis_a(gpt, transformer).\nrelated_to(transformer, attention).")

	// Create in-memory store
	mockStore := newMockStoreForE2E()

	// Create Korel instance
	k := New(Options{
		Store:    mockStore,
		Pipeline: pipeline,
		Inference: inf,
		Weights: ScoreWeights{
			AlphaPMI:     1.0,
			BetaCats:     0.6,
			GammaRecency: 0.8,
			EtaAuthority: 0.2,
			DeltaLen:     0.05,
			ThetaInfer:   0.3,
		},
		RecencyHalfLife: 14,
	})

	// === Phase 2: Ingest Documents ===

	docs := []IngestDoc{
		{
			URL:         "https://example.com/ml-basics",
			Title:       "Machine Learning Basics",
			Outlet:      "Tech Blog",
			PublishedAt: time.Now().Add(-7 * 24 * time.Hour),
			BodyText:    "Machine learning is a subset of artificial intelligence. It uses neural networks and deep learning techniques.",
			SourceCats:  []string{"ai", "education"},
		},
		{
			URL:         "https://example.com/nn-guide",
			Title:       "Neural Network Architecture Guide",
			Outlet:      "AI Weekly",
			PublishedAt: time.Now().Add(-2 * 24 * time.Hour),
			BodyText:    "Neural networks are the foundation of deep learning. They consist of layers that transform input data.",
			SourceCats:  []string{"ai"},
		},
		{
			URL:         "https://example.com/transformer-release",
			Title:       "New Transformer Model Released",
			Outlet:      "Research News",
			PublishedAt: time.Now().Add(-1 * 24 * time.Hour),
			BodyText:    "Researchers announced a new transformer model based on the attention mechanism. BERT and GPT are popular transformers.",
			SourceCats:  []string{"ai", "research"},
		},
	}

	for _, doc := range docs {
		err := k.Ingest(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to ingest document %s: %v", doc.Title, err)
		}
	}

	t.Logf("✓ Ingested %d documents", len(docs))

	// === Phase 3: Query Parsing ===

	queryParser := query.NewParser(tokenizer, mtParser, taxonomy)
	queryStr := "machine learning neural networks"
	parsedQuery := queryParser.Parse(queryStr)

	t.Logf("✓ Parsed query '%s' → tokens: %v, categories: %v",
		queryStr, parsedQuery.Tokens, parsedQuery.Categories)

	if len(parsedQuery.Tokens) == 0 {
		t.Error("Query should produce tokens")
	}

	if len(parsedQuery.Categories) == 0 {
		t.Error("Query should match categories")
	}

	// === Phase 4: Retrieve Candidates ===

	// Create adapter for query.Retriever
	queryStoreAdapter := &queryStoreAdapter{mockStore}
	retriever := query.NewRetriever(queryStoreAdapter)
	candidates, err := retriever.Retrieve(ctx, parsedQuery, 10)
	if err != nil {
		t.Fatalf("Failed to retrieve candidates: %v", err)
	}

	t.Logf("✓ Retrieved %d candidates", len(candidates))

	if len(candidates) == 0 {
		t.Error("Should retrieve at least one candidate")
	}

	// === Phase 5: Rank Candidates ===

	weights := rank.Weights{
		AlphaPMI:     1.0,
		BetaCats:     0.6,
		GammaRecency: 0.8,
		EtaAuthority: 0.2,
		DeltaLen:     0.05,
	}
	scorer := rank.NewScorer(weights, 14.0)

	pmiFunc := func(qt, dt string) float64 {
		// Simple PMI simulation for testing
		if qt == dt {
			return 2.0 // exact match
		}
		// Co-occurrence heuristic for related terms
		related := map[string][]string{
			"machine learning": {"neural network", "deep learning"},
			"neural network":   {"machine learning", "deep learning"},
			"deep learning":    {"machine learning", "neural network"},
		}
		if rels, ok := related[qt]; ok {
			for _, r := range rels {
				if r == dt {
					return 1.5
				}
			}
		}
		return 0.5
	}

	now := time.Now()
	scoredCandidates := make([]struct {
		Candidate rank.Candidate
		Score     float64
	}, len(candidates))

	for i, cand := range candidates {
		score := scorer.Score(parsedQuery, cand, now, pmiFunc)
		scoredCandidates[i].Candidate = cand
		scoredCandidates[i].Score = score

		t.Logf("  Candidate %d: DocID=%d, Score=%.3f", i+1, cand.DocID, score)
	}

	t.Logf("✓ Ranked %d candidates", len(scoredCandidates))

	// Verify scores are reasonable
	for _, sc := range scoredCandidates {
		if sc.Score < 0 {
			t.Errorf("Score should not be negative, got %.3f for DocID %d", sc.Score, sc.Candidate.DocID)
		}
	}

	// === Phase 6: Generate Card ===

	cardBuilder := cards.New()

	// Convert scored candidates to cards.ScoredDoc
	scoredDocs := make([]cards.ScoredDoc, len(scoredCandidates))
	for i, sc := range scoredCandidates {
		scoredDocs[i] = cards.ScoredDoc{
			Title:  mockStore.getTitleForDoc(sc.Candidate.DocID),
			URL:    mockStore.getURLForDoc(sc.Candidate.DocID),
			Time:   sc.Candidate.PublishedAt,
			Tokens: sc.Candidate.Tokens,
			Breakdown: rank.ScoreBreakdown{
				Total: sc.Score,
			},
		}
	}

	card := cardBuilder.Build("Machine Learning Overview", scoredDocs, parsedQuery, nil)

	t.Logf("✓ Generated card: %s (ID: %s)", card.Title, card.ID)
	t.Logf("  Bullets: %d, Sources: %d", len(card.Bullets), len(card.Sources))

	if card.ID == "" {
		t.Error("Card should have an ID")
	}

	if len(card.ID) != 26 {
		t.Errorf("Card ID should be 26 characters (ULID), got %d", len(card.ID))
	}

	if len(card.Bullets) == 0 {
		t.Error("Card should have bullets")
	}

	if len(card.Sources) == 0 {
		t.Error("Card should have sources")
	}

	t.Log("✓ End-to-end test completed successfully")
}

// mockStoreForE2E is a simple mock store for e2e testing
type mockStoreForE2E struct {
	docs      []store.Doc
	docTitles map[int64]string
	docURLs   map[int64]string
}

func newMockStoreForE2E() *mockStoreForE2E {
	return &mockStoreForE2E{
		docs:      []store.Doc{},
		docTitles: make(map[int64]string),
		docURLs:   make(map[int64]string),
	}
}

func (m *mockStoreForE2E) addDoc(id int64, title, url string, publishedAt time.Time, cats []string, tokens []string) {
	m.docs = append(m.docs, store.Doc{
		ID:          id,
		URL:         url,
		Title:       title,
		PublishedAt: publishedAt,
		Cats:        cats,
		LinksOut:    5,
		Tokens:      tokens,
	})
	m.docTitles[id] = title
	m.docURLs[id] = url
}

func (m *mockStoreForE2E) getTitleForDoc(id int64) string {
	return m.docTitles[id]
}

func (m *mockStoreForE2E) getURLForDoc(id int64) string {
	return m.docURLs[id]
}

func (m *mockStoreForE2E) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]store.Doc, error) {
	// For testing, populate docs on first call
	if len(m.docs) == 0 {
		m.addDoc(1, "Machine Learning Basics", "https://example.com/ml-basics",
			time.Now().Add(-7*24*time.Hour), []string{"ai"}, []string{"machine learning", "neural network"})
		m.addDoc(2, "Neural Network Architecture Guide", "https://example.com/nn-guide",
			time.Now().Add(-2*24*time.Hour), []string{"ai"}, []string{"neural network", "deep learning"})
		m.addDoc(3, "New Transformer Model Released", "https://example.com/transformer-release",
			time.Now().Add(-1*24*time.Hour), []string{"ai", "research"}, []string{"transformer", "bert", "gpt"})
	}

	// Simple token matching
	results := []store.Doc{}
	for _, doc := range m.docs {
		for _, qt := range tokens {
			for _, dt := range doc.Tokens {
				if qt == dt {
					results = append(results, doc)
					goto nextDoc
				}
			}
		}
	nextDoc:
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (m *mockStoreForE2E) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {
	// Return related tokens for testing
	neighbors := map[string][]store.Neighbor{
		"machine learning": {
			{Token: "neural network", PMI: 2.5},
			{Token: "deep learning", PMI: 2.0},
		},
		"neural network": {
			{Token: "machine learning", PMI: 2.5},
			{Token: "deep learning", PMI: 1.8},
		},
	}

	if n, ok := neighbors[token]; ok {
		if len(n) > k {
			return n[:k], nil
		}
		return n, nil
	}

	return []store.Neighbor{}, nil
}

// Implement remaining store.Store interface methods (stubs for testing)

func (m *mockStoreForE2E) Close() error { return nil }

func (m *mockStoreForE2E) UpsertDoc(ctx context.Context, d store.Doc) error {
	m.addDoc(d.ID, d.Title, d.URL, d.PublishedAt, d.Cats, d.Tokens)
	return nil
}

func (m *mockStoreForE2E) GetDoc(ctx context.Context, id int64) (store.Doc, error) {
	for _, doc := range m.docs {
		if doc.ID == id {
			return store.Doc{
				ID:          doc.ID,
				URL:         doc.URL,
				Title:       doc.Title,
				PublishedAt: doc.PublishedAt,
				Cats:        doc.Cats,
				LinksOut:    doc.LinksOut,
				Tokens:      doc.Tokens,
			}, nil
		}
	}
	return store.Doc{}, nil
}

func (m *mockStoreForE2E) GetDocByURL(ctx context.Context, url string) (store.Doc, bool, error) {
	return store.Doc{}, false, nil
}

func (m *mockStoreForE2E) UpsertTokenDF(ctx context.Context, token string, df int64) error {
	return nil
}

func (m *mockStoreForE2E) GetTokenDF(ctx context.Context, token string) (int64, error) {
	return 10, nil
}

func (m *mockStoreForE2E) IncPair(ctx context.Context, t1, t2 string) error {
	return nil
}

func (m *mockStoreForE2E) DecPair(ctx context.Context, t1, t2 string) error {
	return nil
}

func (m *mockStoreForE2E) GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error) {
	return 1.5, true, nil
}

func (m *mockStoreForE2E) UpsertCard(ctx context.Context, c store.Card) error {
	return nil
}

func (m *mockStoreForE2E) GetCardsByPeriod(ctx context.Context, period string, k int) ([]store.Card, error) {
	return []store.Card{}, nil
}

func (m *mockStoreForE2E) Stoplist() store.StoplistView {
	return nil
}

func (m *mockStoreForE2E) Dict() store.DictView {
	return nil
}

func (m *mockStoreForE2E) Taxonomy() store.TaxonomyView {
	return nil
}

// queryStoreAdapter adapts store.Store to query.Store interface
type queryStoreAdapter struct {
	*mockStoreForE2E
}

func (a *queryStoreAdapter) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]query.StoreDoc, error) {
	storeDocs, err := a.mockStoreForE2E.GetDocsByTokens(ctx, tokens, limit)
	if err != nil {
		return nil, err
	}

	// Convert store.Doc to query.StoreDoc
	queryDocs := make([]query.StoreDoc, len(storeDocs))
	for i, d := range storeDocs {
		queryDocs[i] = query.StoreDoc{
			ID:          d.ID,
			URL:         d.URL,
			Title:       d.Title,
			PublishedAt: d.PublishedAt,
			Cats:        d.Cats,
			LinksOut:    d.LinksOut,
			Tokens:      d.Tokens,
		}
	}

	return queryDocs, nil
}

func (a *queryStoreAdapter) TopNeighbors(ctx context.Context, token string, k int) ([]query.Neighbor, error) {
	storeNeighbors, err := a.mockStoreForE2E.TopNeighbors(ctx, token, k)
	if err != nil {
		return nil, err
	}

	// Convert store.Neighbor to query.Neighbor
	queryNeighbors := make([]query.Neighbor, len(storeNeighbors))
	for i, n := range storeNeighbors {
		queryNeighbors[i] = query.Neighbor{
			Token: n.Token,
			PMI:   n.PMI,
		}
	}

	return queryNeighbors, nil
}
