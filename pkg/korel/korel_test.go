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
