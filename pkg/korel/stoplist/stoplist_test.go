package stoplist

import (
	"testing"
)

func TestManagerBasic(t *testing.T) {
	stops := []string{"the", "a", "and"}
	mgr := NewManager(stops)

	if !mgr.IsStop("the") {
		t.Error("'the' should be a stopword")
	}

	if mgr.IsStop("hello") {
		t.Error("'hello' should not be a stopword")
	}
}

func TestManagerAddRemove(t *testing.T) {
	mgr := NewManager([]string{"the"})

	// Add new stopword
	mgr.Add("test", Reason{HighDF: true})

	if !mgr.IsStop("test") {
		t.Error("'test' should be stopword after adding")
	}

	// Remove stopword
	mgr.Remove("test")

	if mgr.IsStop("test") {
		t.Error("'test' should not be stopword after removing")
	}
}

func TestManagerAll(t *testing.T) {
	stops := []string{"a", "the", "and"}
	mgr := NewManager(stops)

	all := mgr.All()

	if len(all) != 3 {
		t.Errorf("Expected 3 stopwords, got %d", len(all))
	}

	// Check all are present
	found := make(map[string]bool)
	for _, s := range all {
		found[s] = true
	}

	for _, expected := range stops {
		if !found[expected] {
			t.Errorf("Expected to find '%s' in All()", expected)
		}
	}
}

func TestSuggestCandidates(t *testing.T) {
	mgr := NewManager([]string{})

	stats := []Stats{
		{Token: "the", DFPercent: 90, PMIMax: 0.05, CatEntropy: 0.95},  // Should be candidate
		{Token: "solar", DFPercent: 20, PMIMax: 1.8, CatEntropy: 0.3},  // Should NOT
		{Token: "said", DFPercent: 85, PMIMax: 0.08, CatEntropy: 0.88}, // Should be candidate
		{Token: "python", DFPercent: 40, PMIMax: 1.5, CatEntropy: 0.5}, // Should NOT
	}

	thresholds := Thresholds{
		DFPercent:  80.0,
		PMIMax:     0.1,
		CatEntropy: 0.8,
	}

	candidates := mgr.SuggestCandidates(stats, thresholds)

	// Should suggest "the" and "said"
	if len(candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(candidates))
	}

	expectedTokens := map[string]bool{"the": true, "said": true}
	for _, cand := range candidates {
		if !expectedTokens[cand.Token] {
			t.Errorf("Unexpected candidate: %s", cand.Token)
		}
		if !cand.Reason.HighDF || !cand.Reason.LowPMI || !cand.Reason.HighEntropy {
			t.Error("All three criteria should be true for candidates")
		}
	}
}

func TestSuggestCandidatesSkipsExisting(t *testing.T) {
	mgr := NewManager([]string{"the"})

	stats := []Stats{
		{Token: "the", DFPercent: 90, PMIMax: 0.05, CatEntropy: 0.95},
	}

	thresholds := DefaultThresholds()
	candidates := mgr.SuggestCandidates(stats, thresholds)

	// Should not suggest "the" since it's already a stopword
	if len(candidates) != 0 {
		t.Error("Should not suggest existing stopwords")
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresh := DefaultThresholds()

	if thresh.DFPercent != 80.0 {
		t.Error("Default DF threshold should be 80%")
	}
	if thresh.PMIMax != 0.1 {
		t.Error("Default PMI threshold should be 0.1")
	}
	if thresh.CatEntropy != 0.8 {
		t.Error("Default entropy threshold should be 0.8")
	}
}

func TestCandidateScore(t *testing.T) {
	mgr := NewManager([]string{})

	stats := []Stats{
		{Token: "very-generic", DFPercent: 95, PMIMax: 0.02, CatEntropy: 0.98},
		{Token: "somewhat-generic", DFPercent: 82, PMIMax: 0.09, CatEntropy: 0.85},
	}

	thresholds := DefaultThresholds()
	candidates := mgr.SuggestCandidates(stats, thresholds)

	// Both should be candidates, but first should have higher score
	if len(candidates) < 2 {
		t.Fatal("Expected at least 2 candidates")
	}

	// Find scores
	var score1, score2 float64
	for _, cand := range candidates {
		if cand.Token == "very-generic" {
			score1 = cand.Score
		} else if cand.Token == "somewhat-generic" {
			score2 = cand.Score
		}
	}

	if score1 <= score2 {
		t.Error("More generic token should have higher confidence score")
	}
}

func TestSuggestCandidatesBootstrap(t *testing.T) {
	mgr := NewManager([]string{})
	stats := []Stats{
		{Token: "source", DFPercent: 95, PMIMax: 0, CatEntropy: 0.35},
		{Token: "https", DFPercent: 90, PMIMax: 0, CatEntropy: 0.2},
	}

	candidates := mgr.SuggestCandidates(stats, DefaultThresholds())
	if len(candidates) != 2 {
		t.Fatalf("expected 2 bootstrap candidates, got %d", len(candidates))
	}
	for _, cand := range candidates {
		if !cand.Reason.LowPMI || !cand.Reason.HighDF {
			t.Fatalf("bootstrap reason flags not set for %s: %+v", cand.Token, cand.Reason)
		}
	}
}

func TestEmptyManager(t *testing.T) {
	mgr := NewManager([]string{})

	if mgr.IsStop("anything") {
		t.Error("Empty manager should have no stopwords")
	}

	if len(mgr.All()) != 0 {
		t.Error("Empty manager should return empty list")
	}
}
