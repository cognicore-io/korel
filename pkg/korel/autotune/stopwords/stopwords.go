package stopwords

import (
	"context"
	"errors"

	"github.com/cognicore/korel/pkg/korel/stoplist"
)

// StatsProvider exposes the aggregated metrics required for stopword tuning.
type StatsProvider interface {
	StopwordStats(ctx context.Context) ([]stoplist.Stats, error)
}

// Reviewer optionally performs an extra approval step (human or LLM).
type Reviewer interface {
	Approve(ctx context.Context, cand stoplist.Candidate) (bool, error)
}

// AutoTuner produces ranked stopword suggestions from corpus statistics.
type AutoTuner struct {
	Provider   StatsProvider
	Manager    *stoplist.Manager
	Thresholds stoplist.Thresholds
	Reviewer   Reviewer // optional
}

// Run collects stats, produces candidates, optionally routes them through the reviewer,
// and returns approved suggestions.
func (t *AutoTuner) Run(ctx context.Context) ([]stoplist.Candidate, error) {
	if t.Provider == nil {
		return nil, errors.New("stopwords autotune: nil stats provider")
	}
	if t.Manager == nil {
		return nil, errors.New("stopwords autotune: nil manager")
	}

	stats, err := t.Provider.StopwordStats(ctx)
	if err != nil {
		return nil, err
	}

	candidates := t.Manager.SuggestCandidates(stats, t.thresholdsOrDefault())
	if len(candidates) == 0 || t.Reviewer == nil {
		return candidates, nil
	}

	var approved []stoplist.Candidate
	for _, cand := range candidates {
		ok, err := t.Reviewer.Approve(ctx, cand)
		if err != nil {
			return nil, err
		}
		if ok {
			approved = append(approved, cand)
		}
	}
	return approved, nil
}

func (t *AutoTuner) thresholdsOrDefault() stoplist.Thresholds {
	if t.Thresholds == (stoplist.Thresholds{}) {
		return stoplist.DefaultThresholds()
	}
	return t.Thresholds
}
