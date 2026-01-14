package entities

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	stats []MentionStats
	err   error
}

func (f fakeProvider) EntityMentions(ctx context.Context) ([]MentionStats, error) {
	return f.stats, f.err
}

type fakeReviewer struct {
	ok  map[string]bool
	err error
}

func (f fakeReviewer) ApproveEntity(ctx context.Context, sugg Suggestion) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.ok[sugg.Name], nil
}

func TestEntitiesAutoTuner_NoReviewer(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []MentionStats{
			{Type: "company", Name: "OpenAI", Variant: "openai", Occurrences: 10, Confidence: 0.9},
			{Type: "company", Name: "SmallCo", Variant: "smallco", Occurrences: 1, Confidence: 0.7},
		}},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Name != "OpenAI" {
		t.Fatalf("expected OpenAI suggestion, got %+v", suggestions)
	}
}

func TestEntitiesAutoTuner_WithReviewer(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []MentionStats{
			{Type: "product", Name: "Model-X", Variant: "model x", Occurrences: 5, Confidence: 0.8},
			{Type: "product", Name: "Legacy", Variant: "legacy system", Occurrences: 4, Confidence: 0.7},
		}},
		Reviewer: fakeReviewer{
			ok: map[string]bool{
				"Model-X": true,
				"Legacy":  false,
			},
		},
	}

	suggestions, err := tuner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Name != "Model-X" {
		t.Fatalf("expected reviewer to approve only Model-X, got %+v", suggestions)
	}
}

func TestEntitiesAutoTuner_ProviderError(t *testing.T) {
	tuner := AutoTuner{Provider: fakeProvider{err: errors.New("fail")}}
	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestEntitiesAutoTuner_ReviewerError(t *testing.T) {
	tuner := AutoTuner{
		Provider: fakeProvider{stats: []MentionStats{{Type: "company", Name: "Foo", Variant: "foo", Occurrences: 5, Confidence: 0.8}}},
		Reviewer: fakeReviewer{err: errors.New("review fail")},
	}
	if _, err := tuner.Run(context.Background()); err == nil {
		t.Fatal("expected reviewer error")
	}
}
