package rank

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestScorerBasic(t *testing.T) {
	weights := Weights{
		AlphaPMI:     1.0,
		BetaCats:     0.6,
		GammaRecency: 0.8,
		EtaAuthority: 0.2,
		DeltaLen:     0.05,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{
		Tokens:     []string{"machine-learning"},
		Categories: []string{"ai"},
	}

	candidate := Candidate{
		DocID:       1,
		Tokens:      []string{"machine-learning", "python"},
		Categories:  []string{"ai", "programming"},
		PublishedAt: time.Now().Add(-7 * 24 * time.Hour), // 7 days ago
		LinksOut:    5,
	}

	pmiFunc := func(qt, dt string) float64 {
		if qt == "machine-learning" && dt == "machine-learning" {
			return 2.0
		}
		return 0.0
	}

	score := scorer.Score(query, candidate, time.Now(), pmiFunc)

	if score <= 0 {
		t.Errorf("Score should be positive, got %f", score)
	}
}

func TestScorerRecencyDecay(t *testing.T) {
	weights := Weights{
		AlphaPMI:     0,
		BetaCats:     0,
		GammaRecency: 1.0,
		EtaAuthority: 0,
		DeltaLen:     0,
	}
	scorer := NewScorer(weights, 14.0) // 14-day half-life

	query := Query{}
	now := time.Now()

	// Recent document
	recent := Candidate{PublishedAt: now.Add(-1 * 24 * time.Hour)}
	// Old document
	old := Candidate{PublishedAt: now.Add(-30 * 24 * time.Hour)}

	scoreRecent := scorer.Score(query, recent, now, func(string, string) float64 { return 0 })
	scoreOld := scorer.Score(query, old, now, func(string, string) float64 { return 0 })

	if scoreRecent <= scoreOld {
		t.Error("Recent documents should score higher than old ones")
	}
}

func TestScorerCategoryOverlap(t *testing.T) {
	weights := Weights{
		AlphaPMI:     0,
		BetaCats:     1.0,
		GammaRecency: 0,
		EtaAuthority: 0,
		DeltaLen:     0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{Categories: []string{"ai", "ml"}}

	// High overlap
	highOverlap := Candidate{Categories: []string{"ai", "ml"}}
	// Low overlap
	lowOverlap := Candidate{Categories: []string{"web"}}

	now := time.Now()
	scoreHigh := scorer.Score(query, highOverlap, now, func(string, string) float64 { return 0 })
	scoreLow := scorer.Score(query, lowOverlap, now, func(string, string) float64 { return 0 })

	if scoreHigh <= scoreLow {
		t.Error("High category overlap should score higher")
	}
}

func TestScorerBreakdown(t *testing.T) {
	weights := Weights{
		AlphaPMI:     1.0,
		BetaCats:     0.6,
		GammaRecency: 0.8,
		EtaAuthority: 0.2,
		DeltaLen:     0.05,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{
		Tokens:     []string{"ml"},
		Categories: []string{"ai"},
	}

	candidate := Candidate{
		DocID:       1,
		Tokens:      []string{"ml", "python"},
		Categories:  []string{"ai"},
		PublishedAt: time.Now(),
		LinksOut:    10,
	}

	pmiFunc := func(qt, dt string) float64 { return 1.5 }

	breakdown := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc)

	// Check all components are present
	if breakdown.PMI <= 0 {
		t.Error("PMI component should be positive")
	}
	if breakdown.Cats <= 0 {
		t.Error("Category component should be positive (perfect match)")
	}
	if breakdown.Total == 0 {
		t.Error("Total score should be non-zero")
	}

	// Total should be sum of weighted components
	expected := breakdown.PMI + breakdown.Cats + breakdown.Recency + breakdown.Authority - breakdown.Len
	if breakdown.Total != expected {
		t.Errorf("Total should equal sum of components, got %f, expected %f", breakdown.Total, expected)
	}
}

func TestJaccardSimilarity(t *testing.T) {
	// Perfect overlap
	a := []string{"ai", "ml"}
	b := []string{"ai", "ml"}
	sim := jaccard(a, b)
	if sim != 1.0 {
		t.Errorf("Perfect overlap should be 1.0, got %f", sim)
	}

	// No overlap
	c := []string{"ai"}
	d := []string{"web"}
	sim = jaccard(c, d)
	if sim != 0.0 {
		t.Errorf("No overlap should be 0.0, got %f", sim)
	}

	// Partial overlap
	e := []string{"ai", "ml", "nlp"}
	f := []string{"ai", "web"}
	sim = jaccard(e, f)
	// Intersection: {ai}, Union: {ai, ml, nlp, web}
	// Jaccard = 1/4 = 0.25
	if sim != 0.25 {
		t.Errorf("Expected 0.25, got %f", sim)
	}
}

func TestScorerEmptyQuery(t *testing.T) {
	scorer := NewScorer(Weights{AlphaPMI: 1.0}, 14.0)

	query := Query{}
	candidate := Candidate{Tokens: []string{"test"}}

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 0 })

	// Should not crash, return some score
	_ = score
}

// Edge case tests

func TestScorerFutureTimestamp(t *testing.T) {
	weights := Weights{
		GammaRecency: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{}
	now := time.Now()
	future := now.Add(30 * 24 * time.Hour) // 30 days in future

	candidate := Candidate{PublishedAt: future}

	score := scorer.Score(query, candidate, now, func(string, string) float64 { return 0 })

	// Should handle future timestamps gracefully
	_ = score
}

func TestScorerZeroHalfLife(t *testing.T) {
	weights := Weights{
		GammaRecency: 1.0,
	}
	scorer := NewScorer(weights, 0.0) // zero half-life

	query := Query{}
	candidate := Candidate{PublishedAt: time.Now().Add(-7 * 24 * time.Hour)}

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 0 })

	// Should handle zero half-life without division by zero
	_ = score
}

func TestScorerNegativeAuthority(t *testing.T) {
	weights := Weights{
		EtaAuthority: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{}
	candidate := Candidate{LinksOut: -10} // negative links

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 0 })

	// Should handle negative authority
	_ = score
}

func TestScorerEmptyTokenLists(t *testing.T) {
	weights := Weights{
		AlphaPMI: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{}}
	candidate := Candidate{Tokens: []string{}}

	pmiFunc := func(qt, dt string) float64 { return 1.0 }

	score := scorer.Score(query, candidate, time.Now(), pmiFunc)

	// Should handle empty token lists
	if score < 0 {
		t.Error("Score should not be negative")
	}
}

func TestScorerVeryLargeAuthority(t *testing.T) {
	weights := Weights{
		EtaAuthority: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{}
	candidate := Candidate{LinksOut: 1000000} // very high authority

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 0 })

	// Should handle large authority values
	if score < 0 {
		t.Error("Score should not be negative")
	}
}

func TestScorerAllWeightsZero(t *testing.T) {
	weights := Weights{
		AlphaPMI:     0,
		BetaCats:     0,
		GammaRecency: 0,
		EtaAuthority: 0,
		DeltaLen:     0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"test"}}
	candidate := Candidate{Tokens: []string{"test"}}

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 5.0 })

	// With all weights zero, score should be 0 or minimal
	if score != 0 {
		t.Logf("All zero weights produces score: %f", score)
	}
}

func TestScorerVeryOldDocument(t *testing.T) {
	weights := Weights{
		GammaRecency: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{}
	veryOld := time.Now().Add(-365 * 10 * 24 * time.Hour) // 10 years ago

	candidate := Candidate{PublishedAt: veryOld}

	score := scorer.Score(query, candidate, time.Now(), func(string, string) float64 { return 0 })

	// Very old documents should have very low recency score
	if score > 0.1 {
		t.Errorf("Very old document should have low score, got %f", score)
	}
}

func TestJaccardEmptySets(t *testing.T) {
	// Both empty - implementation returns 1.0 (two empty sets are identical)
	sim := jaccard([]string{}, []string{})
	if sim != 1.0 {
		t.Errorf("Two empty sets should have 1.0 similarity, got %f", sim)
	}

	// One empty - no overlap
	sim = jaccard([]string{"ai"}, []string{})
	if sim != 0.0 {
		t.Errorf("One empty set should have 0 similarity, got %f", sim)
	}
}

func TestJaccardDuplicateElements(t *testing.T) {
	// Test with duplicates in sets
	a := []string{"ai", "ai", "ml", "ml"}
	b := []string{"ai", "web"}

	sim := jaccard(a, b)

	// Jaccard treats as sets, so duplicates ignored
	// Intersection: {ai}, Union: {ai, ml, web}
	// Expected: 1/3 ≈ 0.333
	expected := 1.0 / 3.0
	if math.Abs(sim-expected) > 0.01 {
		t.Errorf("Expected %f, got %f", expected, sim)
	}
}

func TestScorerPMIFunctionReturnsNaN(t *testing.T) {
	weights := Weights{
		AlphaPMI: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"test"}}
	candidate := Candidate{Tokens: []string{"test"}}

	pmiFunc := func(qt, dt string) float64 {
		return math.NaN()
	}

	score := scorer.Score(query, candidate, time.Now(), pmiFunc)

	// Should handle NaN from PMI function
	_ = score
}

func TestScorerPMIFunctionReturnsInf(t *testing.T) {
	weights := Weights{
		AlphaPMI: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"test"}}
	candidate := Candidate{Tokens: []string{"test"}}

	pmiFunc := func(qt, dt string) float64 {
		return math.Inf(1)
	}

	score := scorer.Score(query, candidate, time.Now(), pmiFunc)

	// Should handle Inf from PMI function
	_ = score
}

func TestScorerManyTokens(t *testing.T) {
	weights := Weights{
		AlphaPMI: 1.0,
	}
	scorer := NewScorer(weights, 14.0)

	// Many query tokens
	queryTokens := make([]string, 100)
	for i := 0; i < 100; i++ {
		queryTokens[i] = fmt.Sprintf("token%d", i)
	}

	// Many candidate tokens
	candidateTokens := make([]string, 100)
	for i := 0; i < 100; i++ {
		candidateTokens[i] = fmt.Sprintf("token%d", i)
	}

	query := Query{Tokens: queryTokens}
	candidate := Candidate{Tokens: candidateTokens}

	pmiFunc := func(qt, dt string) float64 { return 1.0 }

	score := scorer.Score(query, candidate, time.Now(), pmiFunc)

	// Should handle many tokens without performance issues
	if score <= 0 {
		t.Error("Score with many matching tokens should be positive")
	}
}

func TestScorerDampingReducesPMI(t *testing.T) {
	weights := Weights{AlphaPMI: 1.0}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"hub-token"}}
	candidate := Candidate{
		Tokens:      []string{"hub-token"},
		PublishedAt: time.Now(),
	}
	pmiFunc := func(qt, dt string) float64 { return 2.0 }

	// Without damping.
	undamped := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc)

	// With damping: hub-token gets 0.1 factor.
	dampMap := map[string]float64{"hub-token": 0.1}
	damped := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc, dampMap)

	// Damped PMI should be ~10% of undamped.
	ratio := damped.PMI / undamped.PMI
	if math.Abs(ratio-0.1) > 0.01 {
		t.Errorf("expected damped PMI to be ~10%% of undamped, got ratio %.3f (damped=%.3f, undamped=%.3f)",
			ratio, damped.PMI, undamped.PMI)
	}

	// Damping field should reflect the applied factor.
	if math.Abs(damped.Damping-0.1) > 0.01 {
		t.Errorf("expected Damping=0.1, got %.3f", damped.Damping)
	}
}

func TestScorerDampingNonHubUnaffected(t *testing.T) {
	weights := Weights{AlphaPMI: 1.0}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"normal-token"}}
	candidate := Candidate{
		Tokens:      []string{"normal-token"},
		PublishedAt: time.Now(),
	}
	pmiFunc := func(qt, dt string) float64 { return 2.0 }

	// Damping map with no entry for "normal-token" — defaults to 1.0.
	dampMap := map[string]float64{"other": 0.1}
	damped := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc, dampMap)
	undamped := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc)

	if math.Abs(damped.PMI-undamped.PMI) > 0.001 {
		t.Errorf("non-hub token should have same PMI: damped=%.3f undamped=%.3f", damped.PMI, undamped.PMI)
	}
}

func TestScorerDampingMultipleTokens(t *testing.T) {
	weights := Weights{AlphaPMI: 1.0}
	scorer := NewScorer(weights, 14.0)

	query := Query{Tokens: []string{"hub", "normal"}}
	candidate := Candidate{
		Tokens:      []string{"hub", "normal"},
		PublishedAt: time.Now(),
	}
	pmiFunc := func(qt, dt string) float64 {
		if qt == dt {
			return 2.0
		}
		return 0.0
	}

	dampMap := map[string]float64{"hub": 0.2} // normal defaults to 1.0

	damped := scorer.ScoreWithBreakdown(query, candidate, time.Now(), pmiFunc, dampMap)

	// hub contributes 2.0 * 0.2 = 0.4, normal contributes 2.0 * 1.0 = 2.0
	// Average PMI = (0.4 + 2.0) / 2 = 1.2 → weighted = 1.0 * 1.2 = 1.2
	expected := 1.2
	if math.Abs(damped.PMI-expected) > 0.01 {
		t.Errorf("expected PMI=%.2f, got %.3f", expected, damped.PMI)
	}

	// Average damping = (0.2 + 1.0) / 2 = 0.6
	if math.Abs(damped.Damping-0.6) > 0.01 {
		t.Errorf("expected Damping=0.6, got %.3f", damped.Damping)
	}
}

func TestScorerBreakdownComponentsSum(t *testing.T) {
	weights := Weights{
		AlphaPMI:     1.0,
		BetaCats:     0.5,
		GammaRecency: 0.8,
		EtaAuthority: 0.2,
		DeltaLen:     0.1,
	}
	scorer := NewScorer(weights, 14.0)

	query := Query{
		Tokens:     []string{"ml"},
		Categories: []string{"ai"},
	}

	candidate := Candidate{
		Tokens:      []string{"ml"},
		Categories:  []string{"ai"},
		PublishedAt: time.Now(),
		LinksOut:    10,
	}

	breakdown := scorer.ScoreWithBreakdown(query, candidate, time.Now(), func(string, string) float64 { return 2.0 })

	// Verify total equals sum of components
	calculated := breakdown.PMI + breakdown.Cats + breakdown.Recency + breakdown.Authority - breakdown.Len
	if math.Abs(breakdown.Total-calculated) > 0.001 {
		t.Errorf("Total (%f) should equal sum of components (%f)", breakdown.Total, calculated)
	}
}
