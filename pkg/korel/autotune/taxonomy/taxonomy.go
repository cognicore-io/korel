package taxonomy

import (
	"context"
	"errors"
	"math"
)

// DriftStats captures analytics for category keyword coverage/usage.
type DriftStats struct {
	Category    string
	Keyword     string
	SupportDocs int64   // number of docs where keyword appears
	MissedDocs  int64   // docs tagged with category but missing keyword
	Coverage    float64 // % of category docs containing keyword
}

// Suggestion represents a proposed keyword addition or boost.
type Suggestion struct {
	Category   string
	Keyword    string
	Confidence float64
	MissedDocs int64
}

// StatsProvider supplies drift metrics.
type StatsProvider interface {
	TaxonomyDrift(ctx context.Context) ([]DriftStats, error)
}

// Reviewer optionally approves taxonomy suggestions.
type Reviewer interface {
	ApproveTaxonomy(ctx context.Context, sugg Suggestion) (bool, error)
}

// Thresholds control sensitivity.
type Thresholds struct {
	MinCoverage    float64 // e.g. 0.3 (30% coverage)
	MinMissedDocs  int64   // e.g. 20 docs lacking the keyword
	ConfidenceBias float64 // default boost for confidence
}

// AutoTuner generates taxonomy keyword suggestions.
type AutoTuner struct {
	Provider   StatsProvider
	Thresholds Thresholds
	Reviewer   Reviewer // optional
}

// Run executes the autotuner and returns approved suggestions.
func (t *AutoTuner) Run(ctx context.Context) ([]Suggestion, error) {
	if t.Provider == nil {
		return nil, errors.New("taxonomy autotune: nil stats provider")
	}
	stats, err := t.Provider.TaxonomyDrift(ctx)
	if err != nil {
		return nil, err
	}

	thresholds := t.thresholdsOrDefault()
	var suggestions []Suggestion
	for _, stat := range stats {
		if stat.Coverage >= thresholds.MinCoverage {
			continue
		}
		if stat.MissedDocs < thresholds.MinMissedDocs {
			continue
		}
		confidence := t.computeConfidence(stat, thresholds)
		suggestions = append(suggestions, Suggestion{
			Category:   stat.Category,
			Keyword:    stat.Keyword,
			Confidence: confidence,
			MissedDocs: stat.MissedDocs,
		})
	}

	if t.Reviewer == nil {
		return suggestions, nil
	}

	var approved []Suggestion
	for _, sugg := range suggestions {
		ok, err := t.Reviewer.ApproveTaxonomy(ctx, sugg)
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
	if th.MinCoverage == 0 {
		th.MinCoverage = 0.4
	}
	if th.MinMissedDocs == 0 {
		th.MinMissedDocs = 10
	}
	if th.ConfidenceBias == 0 {
		th.ConfidenceBias = 0.2
	}
	return th
}

func (t *AutoTuner) computeConfidence(stat DriftStats, th Thresholds) float64 {
	missedComponent := 1 - math.Exp(-float64(stat.MissedDocs)/float64(th.MinMissedDocs))
	coverageComponent := 1 - stat.Coverage // more missing coverage => higher urgency
	confidence := th.ConfidenceBias + 0.5*missedComponent + 0.5*coverageComponent
	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0 {
		confidence = 0
	}
	return confidence
}
