package entities

import (
	"context"
	"errors"
)

// MentionStats describes observed entity mentions in text.
type MentionStats struct {
	Type        string
	Name        string
	Variant     string
	Occurrences int64
	Confidence  float64
}

// Suggestion represents a proposed taxonomy entity entry.
type Suggestion struct {
	Type       string
	Name       string
	Variant    string
	Confidence float64
}

// StatsProvider supplies entity mention stats (e.g., from NLP tagging).
type StatsProvider interface {
	EntityMentions(ctx context.Context) ([]MentionStats, error)
}

// Reviewer optionally approves entity additions.
type Reviewer interface {
	ApproveEntity(ctx context.Context, sugg Suggestion) (bool, error)
}

// Thresholds control auto-approval sensitivity.
type Thresholds struct {
	MinOccurrences int64
	MinConfidence  float64
}

// AutoTuner suggests new dictionary entities.
type AutoTuner struct {
	Provider   StatsProvider
	Thresholds Thresholds
	Reviewer   Reviewer
}

func (t *AutoTuner) Run(ctx context.Context) ([]Suggestion, error) {
	if t.Provider == nil {
		return nil, errors.New("entities autotune: nil stats provider")
	}
	stats, err := t.Provider.EntityMentions(ctx)
	if err != nil {
		return nil, err
	}
	th := t.thresholdsOrDefault()

	var suggestions []Suggestion
	for _, stat := range stats {
		if stat.Occurrences < th.MinOccurrences {
			continue
		}
		if stat.Confidence < th.MinConfidence {
			continue
		}
		suggestions = append(suggestions, Suggestion{
			Type:       stat.Type,
			Name:       stat.Name,
			Variant:    stat.Variant,
			Confidence: stat.Confidence,
		})
	}

	if t.Reviewer == nil {
		return suggestions, nil
	}

	var approved []Suggestion
	for _, s := range suggestions {
		ok, err := t.Reviewer.ApproveEntity(ctx, s)
		if err != nil {
			return nil, err
		}
		if ok {
			approved = append(approved, s)
		}
	}
	return approved, nil
}

func (t *AutoTuner) thresholdsOrDefault() Thresholds {
	th := t.Thresholds
	if th.MinOccurrences == 0 {
		th.MinOccurrences = 3
	}
	if th.MinConfidence == 0 {
		th.MinConfidence = 0.6
	}
	return th
}
