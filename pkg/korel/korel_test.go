package korel

import (
	"context"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/memstore"
)

func TestIngestReingestAdjustsStats(t *testing.T) {
	ctx := context.Background()

	store := memstore.New()
	defer store.Close()

	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer([]string{}),
		ingest.NewMultiTokenParser([]ingest.DictEntry{}),
		ingest.NewTaxonomy(),
	)

	engine := New(Options{
		Store:     store,
		Pipeline:  pipeline,
		Inference: simple.New(),
		Weights: ScoreWeights{
			AlphaPMI:     1,
			BetaCats:     0,
			GammaRecency: 0,
			EtaAuthority: 0,
			DeltaLen:     0,
			ThetaInfer:   0,
		},
		RecencyHalfLife: 14,
	})
	defer engine.Close()

	doc := IngestDoc{
		URL:         "https://example.com/doc-1",
		Title:       "Doc",
		PublishedAt: time.Now(),
		BodyText:    "alpha beta beta",
	}

	if err := engine.Ingest(ctx, doc); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	assertTokenDF(t, ctx, engine, "alpha", 1)
	assertTokenDF(t, ctx, engine, "beta", 1)
	assertTokenDF(t, ctx, engine, "gamma", 0)
	assertPairExists(t, ctx, engine, "alpha", "beta", true)

	// Re-ingest same URL with different body
	doc.BodyText = "alpha gamma"
	if err := engine.Ingest(ctx, doc); err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	assertTokenDF(t, ctx, engine, "alpha", 1)
	assertTokenDF(t, ctx, engine, "beta", 0)
	assertTokenDF(t, ctx, engine, "gamma", 1)
	assertPairExists(t, ctx, engine, "alpha", "beta", false)
	assertPairExists(t, ctx, engine, "alpha", "gamma", true)
}

func TestAutoTunePersistAndRebuild(t *testing.T) {
	ctx := context.Background()

	ms := memstore.New()
	defer ms.Close()

	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer(nil),
		ingest.NewMultiTokenParser(nil),
		ingest.NewTaxonomy(),
	)

	engine := New(Options{
		Store:           ms,
		Pipeline:        pipeline,
		Inference:       simple.New(),
		Weights:         ScoreWeights{AlphaPMI: 1},
		RecencyHalfLife: 14,
	})
	defer engine.Close()

	// Build a small corpus where "the" appears in every doc (obvious stopword).
	// Other tokens are topical and should NOT be flagged.
	corpus := []string{
		"the cat sat on the mat",
		"the dog chased the ball",
		"the fish swam in the sea",
		"the bird flew over the tree",
		"the cat played with the yarn",
		"the dog fetched the stick",
		"the fish hid under the rock",
		"the bird sang in the rain",
		"the cat napped on the couch",
		"the dog ran through the park",
	}

	result, err := engine.AutoTune(ctx, corpus, &AutoTuneOptions{
		MaxIterations: 2,
		Thresholds:    AutoTuneDefaults(),
	})
	if err != nil {
		t.Fatalf("AutoTune: %v", err)
	}

	// "the" should be discovered as a stopword (appears in 100% of docs).
	foundThe := false
	for _, c := range result.StopwordCandidates {
		if c.Token == "the" {
			foundThe = true
			break
		}
	}
	if !foundThe {
		t.Fatal("expected 'the' to be discovered as a stopword")
	}

	// Verify persistence: store should have a non-nil Stoplist view.
	sl := ms.Stoplist()
	if sl == nil {
		t.Fatal("expected Stoplist to be non-nil after AutoTune")
	}
	if !sl.IsStop("the") {
		t.Fatal("expected 'the' in persisted stoplist")
	}

	// RebuildPipeline should pick up the persisted stopwords.
	engine.RebuildPipeline()

	// Ingest a doc with "the" — it should be filtered by the new pipeline.
	if err := engine.Ingest(ctx, IngestDoc{
		URL:         "https://example.com/post-rebuild",
		Title:       "Post Rebuild",
		PublishedAt: time.Now(),
		BodyText:    "the fox jumped over the fence",
	}); err != nil {
		t.Fatalf("Ingest after rebuild: %v", err)
	}

	doc, found, err := ms.GetDocByURL(ctx, "https://example.com/post-rebuild")
	if err != nil || !found {
		t.Fatalf("GetDocByURL: err=%v found=%v", err, found)
	}

	// "the" should NOT be in the stored tokens.
	for _, tok := range doc.Tokens {
		if tok == "the" {
			t.Fatal("expected 'the' to be filtered from tokens after RebuildPipeline")
		}
	}

	// But topical tokens should still be present.
	hasTopical := false
	for _, tok := range doc.Tokens {
		if tok == "fox" || tok == "jumped" || tok == "fence" {
			hasTopical = true
			break
		}
	}
	if !hasTopical {
		t.Fatalf("expected topical tokens in doc, got %v", doc.Tokens)
	}
}

func assertTokenDF(t *testing.T, ctx context.Context, k *Korel, token string, expected int64) {
	t.Helper()
	df, err := k.store.GetTokenDF(ctx, token)
	if err != nil {
		t.Fatalf("GetTokenDF(%s): %v", token, err)
	}
	if df != expected {
		t.Fatalf("token %s df = %d, expected %d", token, df, expected)
	}
}

func assertPairExists(t *testing.T, ctx context.Context, k *Korel, a, b string, expected bool) {
	t.Helper()
	_, ok, err := k.store.GetPMI(ctx, a, b)
	if err != nil {
		t.Fatalf("GetPMI(%s,%s): %v", a, b, err)
	}
	if ok != expected {
		t.Fatalf("pair (%s,%s) existence = %v, expected %v", a, b, ok, expected)
	}
}
