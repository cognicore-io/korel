package taxonomy

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	stats []DriftStats
	err   error
}

func (f fakeProvider) TaxonomyDrift(ctx context.Context) ([]DriftStats, error) {
	return f.stats, f.err
}

type fakeReviewer struct {
	decisions map[string]bool
	err       error
}

func (f fakeReviewer) ApproveTaxonomy(ctx context.Context, sugg Suggestion) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.decisions[sugg.Keyword], nil
}

func TestAutoTunerTaxonomy_NoReviewer(t *testing.T) {
	stats := []DriftStats{
		{Category: "ai", Keyword: "transformer", Coverage: 0.2, MissedDocs: 30},
		{Category: "ai", Keyword: "neural network", Coverage: 0.7, MissedDocs: 5},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Thresholds: Thresholds{
			MinCoverage:   0.5,
			MinMissedDocs: 10,
		},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(suggestions) != 1 || suggestions[0].Keyword != "transformer" {
		t.Fatalf("Expected transformer suggestion, got %+v", suggestions)
	}

	if suggestions[0].Confidence <= 0 {
		t.Fatalf("Confidence should be > 0, got %f", suggestions[0].Confidence)
	}
}

func TestAutoTunerTaxonomy_WithReviewer(t *testing.T) {
	stats := []DriftStats{
		{Category: "energy", Keyword: "solar", Coverage: 0.3, MissedDocs: 40},
		{Category: "energy", Keyword: "battery", Coverage: 0.25, MissedDocs: 15},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Reviewer: fakeReviewer{
			decisions: map[string]bool{
				"solar":   true,
				"battery": false,
			},
		},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(suggestions) != 1 || suggestions[0].Keyword != "solar" {
		t.Fatalf("Expected reviewer to approve only solar, got %+v", suggestions)
	}
}

func TestAutoTunerTaxonomy_ProviderError(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{err: errors.New("db down")},
	}

	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestAutoTunerTaxonomy_ReviewerError(t *testing.T) {
	stats := []DriftStats{
		{Category: "ai", Keyword: "ml", Coverage: 0.3, MissedDocs: 50},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Reviewer: fakeReviewer{err: errors.New("llm timeout")},
	}

	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected reviewer error")
	}
}
