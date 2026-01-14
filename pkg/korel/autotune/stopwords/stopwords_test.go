package stopwords

import (
	"context"
	"errors"
	"testing"

	"github.com/cognicore/korel/pkg/korel/stoplist"
)

type fakeProvider struct {
	stats []stoplist.Stats
	err   error
}

func (f fakeProvider) StopwordStats(ctx context.Context) ([]stoplist.Stats, error) {
	return f.stats, f.err
}

type fakeReviewer struct {
	decisions map[string]bool
	err       error
}

func (f fakeReviewer) Approve(ctx context.Context, cand stoplist.Candidate) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.decisions[cand.Token], nil
}

func TestAutoTunerRun_NoReviewer(t *testing.T) {
	mgr := stoplist.NewManager([]string{})
	stats := []stoplist.Stats{
		{Token: "the", DFPercent: 90, PMIMax: 0.05, CatEntropy: 0.95},
		{Token: "ai", DFPercent: 20, PMIMax: 1.5, CatEntropy: 0.4},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Manager:  mgr,
	}

	ctx := context.Background()
	cands, err := tuner.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(cands) != 1 || cands[0].Token != "the" {
		t.Fatalf("Expected 'the' as candidate, got %+v", cands)
	}
}

func TestAutoTunerRun_WithReviewer(t *testing.T) {
	mgr := stoplist.NewManager([]string{})
	stats := []stoplist.Stats{
		{Token: "and", DFPercent: 85, PMIMax: 0.02, CatEntropy: 0.9},
		{Token: "news", DFPercent: 82, PMIMax: 0.03, CatEntropy: 0.85},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Manager:  mgr,
		Reviewer: fakeReviewer{
			decisions: map[string]bool{
				"and":  true,
				"news": false,
			},
		},
	}

	cands, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(cands) != 1 || cands[0].Token != "and" {
		t.Fatalf("Expected reviewer to approve only 'and', got %+v", cands)
	}
}

func TestAutoTunerRun_ProviderError(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{err: errors.New("boom")},
		Manager:  stoplist.NewManager(nil),
	}

	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected error from provider")
	}
}

func TestAutoTunerRun_ReviewerError(t *testing.T) {
	mgr := stoplist.NewManager([]string{})
	stats := []stoplist.Stats{
		{Token: "the", DFPercent: 90, PMIMax: 0.05, CatEntropy: 0.95},
	}

	tuner := AutoTuner{
		Provider: fakeProvider{stats: stats},
		Manager:  mgr,
		Reviewer: fakeReviewer{err: errors.New("review failed")},
	}

	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected reviewer error")
	}
}
