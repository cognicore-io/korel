package rules

import (
	"context"
	"errors"
	"math"
)

// PairStats describes PMI/DF metrics for a token pair.
type PairStats struct {
	Subject string
	Object  string
	PMI     float64
	Support int64 // number of co-occurrences
}

// Suggestion represents a candidate symbolic relation.
type Suggestion struct {
	Relation   string
	Subject    string
	Object     string
	Confidence float64
	Support    int64
}

// StatsProvider supplies PMI pair stats.
type StatsProvider interface {
	HighPMIPairs(ctx context.Context) ([]PairStats, error)
}

// Reviewer optionally approves a rule suggestion.
type Reviewer interface {
	ApproveRule(ctx context.Context, sugg Suggestion) (bool, error)
}

// Thresholds control sensitivity.
type Thresholds struct {
	MinPMI     float64
	MinSupport int64
	Relation   string
}

// AutoTuner proposes symbolic rules from PMI stats.
type AutoTuner struct {
	Provider   StatsProvider
	Thresholds Thresholds
	Reviewer   Reviewer // optional
}

func (t *AutoTuner) Run(ctx context.Context) ([]Suggestion, error) {
	if t.Provider == nil {
		return nil, errors.New("rules autotune: nil stats provider")
	}
	stats, err := t.Provider.HighPMIPairs(ctx)
	if err != nil {
		return nil, err
	}

	th := t.thresholdsOrDefault()
	var suggestions []Suggestion
	for _, stat := range stats {
		if stat.PMI < th.MinPMI {
			continue
		}
		if stat.Support < th.MinSupport {
			continue
		}
		confidence := sigmoid(stat.PMI-th.MinPMI) * 0.6
		confidence += sigmoid(float64(stat.Support-th.MinSupport)) * 0.4
		suggestions = append(suggestions, Suggestion{
			Relation:   th.Relation,
			Subject:    stat.Subject,
			Object:     stat.Object,
			Confidence: confidence,
			Support:    stat.Support,
		})
	}

	if t.Reviewer == nil {
		return suggestions, nil
	}

	var approved []Suggestion
	for _, sugg := range suggestions {
		ok, err := t.Reviewer.ApproveRule(ctx, sugg)
		if err != nil {
			return nil, err
		}
		if ok {
			approved = append(approved, sugg)
		}
	}
	return approved, nil
}

func (t *AutoTuner) thresholdsOrDefault() Thresholds {
	th := t.Thresholds
	if th.MinPMI == 0 {
		th.MinPMI = 0.8
	}
	if th.MinSupport == 0 {
		th.MinSupport = 5
	}
	if th.Relation == "" {
		th.Relation = "related_to"
	}
	return th
}

func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}
