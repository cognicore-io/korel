package rules

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	stats []PairStats
	err   error
}

func (f fakeProvider) HighPMIPairs(ctx context.Context) ([]PairStats, error) {
	return f.stats, f.err
}

type fakeReviewer struct {
	ok  map[string]bool
	err error
}

func (f fakeReviewer) ApproveRule(ctx context.Context, sugg Suggestion) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.ok[sugg.Subject+"-"+sugg.Object], nil
}

func TestRulesAutoTuner_NoReviewer(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []PairStats{
			{Subject: "machine-learning", Object: "neural-network", PMI: 1.4, Support: 20},
			{Subject: "ai", Object: "finance", PMI: 0.4, Support: 50},
		}},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Subject != "machine-learning" {
		t.Fatalf("expected ML suggestion, got %+v", suggestions)
	}
}

func TestRulesAutoTuner_WithReviewer(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []PairStats{
			{Subject: "solar", Object: "renewables", PMI: 1.2, Support: 15},
			{Subject: "battery", Object: "storage", PMI: 1.0, Support: 12},
		}},
		Reviewer: fakeReviewer{
			ok: map[string]bool{
				"solar-renewables": true,
			},
		},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Subject != "solar" {
		t.Fatalf("expected reviewer to approve only solar pair, got %+v", suggestions)
	}
}

func TestRulesAutoTuner_ProviderError(t *testing.T) {
	tuner := AutoTuner{Provider: fakeProvider{err: errors.New("down")}}
	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestRulesAutoTuner_ReviewerError(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []PairStats{{Subject: "ml", Object: "dl", PMI: 2, Support: 10}}},
		Reviewer: fakeReviewer{err: errors.New("timeout")},
	}
	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected reviewer error")
	}
}
