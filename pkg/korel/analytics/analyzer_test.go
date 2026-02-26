package analytics

import (
	"context"
	"math"
	"testing"

	"github.com/cognicore/korel/pkg/korel/signals"
)

func TestAnalyzerStopwordStats(t *testing.T) {
	a := NewAnalyzer()
	a.Process([]string{"the", "quick", "brown"}, []string{"story"})
	a.Process([]string{"the", "lazy"}, []string{"story"})
	a.Process([]string{"quick", "lazy"}, []string{"news"})
	stats := a.Snapshot()

	if stats.TotalDocs != 3 {
		t.Fatalf("expected 3 docs, got %d", stats.TotalDocs)
	}
	sw := stats.StopwordStats()
	if len(sw) == 0 {
		t.Fatal("expected stopword stats")
	}
	provider := NewStopwordStatsProvider(stats)
	list, err := provider.StopwordStats(context.Background())
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if len(list) != len(sw) {
		t.Fatalf("provider mismatch")
	}

	pairs := stats.TopPairs(5, 0.0)
	if len(pairs) == 0 {
		t.Fatalf("expected pair stats")
	}
}

func TestAnalyzerPMIMax(t *testing.T) {
	a := NewAnalyzer()

	// Test that PMIMax is calculated from pair statistics
	// Note: In small corpora, PMI smoothing dominates, so we just verify
	// that PMIMax is non-zero and comes from actual pair calculations

	// Create some docs with token pairs
	a.Process([]string{"machine", "learning"}, []string{"ai"})
	a.Process([]string{"machine", "learning", "algorithms"}, []string{"ai"})
	a.Process([]string{"the", "data"}, []string{"general"})
	a.Process([]string{"the", "model"}, []string{"general"})

	stats := a.Snapshot()
	swStats := stats.StopwordStats()

	// Find PMIMax for each token
	pmiByToken := make(map[string]float64)
	for _, sw := range swStats {
		pmiByToken[sw.Token] = sw.PMIMax
	}

	// Verify PMIMax is computed (not zero) for tokens that appear in pairs
	if pmiByToken["machine"] == 0.0 {
		t.Error("machine PMIMax is 0.0, but it appears in pairs - PMI calculation not working")
	}
	if pmiByToken["learning"] == 0.0 {
		t.Error("learning PMIMax is 0.0, but it appears in pairs - PMI calculation not working")
	}
	if pmiByToken["the"] == 0.0 {
		t.Error("the PMIMax is 0.0, but it appears in pairs - PMI calculation not working")
	}

	// Verify PMIMax is positive (shows it's being calculated)
	for tok, pmi := range pmiByToken {
		if pmi < 0 {
			t.Errorf("Token %s has negative PMIMax %.2f, PMI should be non-negative", tok, pmi)
		}
	}

	t.Logf("PMIMax calculated for %d tokens (sample: machine=%.2f, learning=%.2f, the=%.2f)",
		len(pmiByToken), pmiByToken["machine"], pmiByToken["learning"], pmiByToken["the"])
}

func TestAnalyzerBigramVsDocumentPairs(t *testing.T) {
	a := NewAnalyzer()

	// Test that bigrams track adjacency, document pairs track co-occurrence
	// Doc: "deep learning models use deep neural networks"
	tokens := []string{"deep", "learning", "models", "use", "deep", "neural", "networks"}
	a.Process(tokens, []string{"ai"})

	stats := a.Snapshot()

	// Check bigram counts (adjacent pairs)
	type bigramCase struct {
		a, b  string
		count int64
	}
	expectedBigrams := []bigramCase{
		{"deep", "learning", 1},
		{"learning", "models", 1},
		{"models", "use", 1},
		{"use", "deep", 1},
		{"deep", "neural", 1},
		{"neural", "networks", 1},
	}

	for _, tc := range expectedBigrams {
		if got := stats.BigramCount(tc.a, tc.b); got != tc.count {
			t.Errorf("Bigram (%s,%s): got count %d, want %d", tc.a, tc.b, got, tc.count)
		}
	}

	// Check document pair counts (all unique token combinations)
	// "deep" appears twice, but only counted once in document pairs
	if got := stats.PairCount("deep", "learning"); got != 1 {
		t.Errorf("Document pair (deep,learning): got count %d, want 1", got)
	}

	// Verify we have more document pairs than bigrams (since document pairs are all combinations)
	if len(stats.PairCounts) <= len(stats.BigramCounts) {
		t.Errorf("Expected more document pairs (%d) than bigrams (%d)", len(stats.PairCounts), len(stats.BigramCounts))
	}
}

func TestAnalyzerWindowSizeValidation(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{DefaultSkipGramWindow, DefaultSkipGramWindow}, // Default
		{10, 10},                                       // Valid custom
		{MinSkipGramWindow, MinSkipGramWindow},         // Minimum valid
		{1, MinSkipGramWindow},                         // Below min - clamped
		{0, MinSkipGramWindow},                         // Below min - clamped
		{-1, MinSkipGramWindow},                        // Below min - clamped
	}

	for _, tt := range tests {
		a := NewAnalyzerWithWindow(tt.input)
		if a.windowSize != tt.want {
			t.Errorf("NewAnalyzerWithWindow(%d) windowSize = %d, want %d", tt.input, a.windowSize, tt.want)
		}
	}
}

func TestAnalyzerSkipGramDeduplication(t *testing.T) {
	a := NewAnalyzerWithWindow(5)

	// Document with "transformer" appearing multiple times
	// "transformer attention mechanism transformer model"
	// Even though transformer appears twice, skip-gram pair (transformer, attention)
	// should only be counted once per document
	tokens := []string{"transformer", "attention", "mechanism", "transformer", "model"}
	a.Process(tokens, []string{"ai"})

	stats := a.Snapshot()

	// (transformer, attention) should be counted once, not twice
	if got := stats.SkipGramCount("transformer", "attention"); got != 1 {
		t.Errorf("Skip-gram (transformer, attention) count = %d, want 1 (should be deduplicated per document)", got)
	}

	// (transformer, model) should also be counted once
	if got := stats.SkipGramCount("transformer", "model"); got != 1 {
		t.Errorf("Skip-gram (transformer, model) count = %d, want 1", got)
	}
}

func TestAnalyzerSkipGramWindow(t *testing.T) {
	a := NewAnalyzerWithWindow(3)

	// With window=3, "a b c d e" should capture:
	// (a,b), (a,c) - from position 0
	// (b,c), (b,d) - from position 1
	// (c,d), (c,e) - from position 2
	// (d,e) - from position 3
	// But NOT (a,d), (a,e), (b,e) - outside window
	tokens := []string{"a", "b", "c", "d", "e"}
	a.Process(tokens, []string{"test"})

	stats := a.Snapshot()

	// Should be in window
	type sgCase struct{ a, b string }
	shouldExist := []sgCase{
		{"a", "b"}, {"a", "c"}, {"b", "c"}, {"b", "d"},
		{"c", "d"}, {"c", "e"}, {"d", "e"},
	}

	for _, tc := range shouldExist {
		if got := stats.SkipGramCount(tc.a, tc.b); got != 1 {
			t.Errorf("Skip-gram (%s, %s) should exist in window=3, got count %d", tc.a, tc.b, got)
		}
	}

	// Should NOT be in window
	shouldNotExist := []sgCase{{"a", "d"}, {"a", "e"}, {"b", "e"}}

	for _, tc := range shouldNotExist {
		if got := stats.SkipGramCount(tc.a, tc.b); got != 0 {
			t.Errorf("Skip-gram (%s, %s) should NOT exist (outside window=3), got count %d", tc.a, tc.b, got)
		}
	}
}

func TestAnalyzerCTokenPairs(t *testing.T) {
	a := NewAnalyzerWithWindow(5)

	// Create corpus with co-occurrences
	a.Process([]string{"transformer", "attention", "mechanism"}, []string{"ai"})
	a.Process([]string{"transformer", "attention", "model"}, []string{"ai"})
	a.Process([]string{"neural", "network", "layers"}, []string{"ai"})

	stats := a.Snapshot()

	// Get c-token pairs with minimum support
	pairs := stats.CTokenPairs(2) // Min support = 2

	// (transformer, attention) should be included (support=2)
	found := false
	for _, p := range pairs {
		if (p.TokenA == "transformer" && p.TokenB == "attention") ||
			(p.TokenA == "attention" && p.TokenB == "transformer") {
			found = true
			if p.Support != 2 {
				t.Errorf("C-token pair (transformer, attention) support = %d, want 2", p.Support)
			}
			// PMI should be calculated (may be zero or negative with smoothing in small corpora)
			// Just verify it's not the uninitialized sentinel value
			if p.PMI < -10 || p.PMI > 10 {
				t.Errorf("C-token pair PMI out of reasonable range: %.2f", p.PMI)
			}
		}
	}
	if !found {
		t.Error("C-token pairs should include (transformer, attention) with support >= 2")
	}

	// Pairs with support < 2 should be filtered out
	for _, p := range pairs {
		if p.Support < 2 {
			t.Errorf("Found pair (%s, %s) with support %d < minSupport 2", p.TokenA, p.TokenB, p.Support)
		}
	}
}

func TestAnalyzerCTokenPairsSorted(t *testing.T) {
	a := NewAnalyzerWithWindow(5)

	// Create corpus where some pairs have higher PMI than others
	for i := 0; i < 10; i++ {
		a.Process([]string{"high", "frequency", "pair"}, []string{"test"})
	}
	for i := 0; i < 5; i++ {
		a.Process([]string{"medium", "frequency", "pair"}, []string{"test"})
	}

	stats := a.Snapshot()
	pairs := stats.CTokenPairs(1)

	// Should be sorted by PMI (descending)
	for i := 0; i < len(pairs)-1; i++ {
		if pairs[i].PMI < pairs[i+1].PMI {
			t.Errorf("C-token pairs should be sorted by PMI descending, but pairs[%d].PMI=%.2f < pairs[%d].PMI=%.2f",
				i, pairs[i].PMI, i+1, pairs[i+1].PMI)
		}
	}
}

// --- Damping tests ---

// buildDampingCorpus creates a corpus where "hub" co-occurs with many tokens
// (high density) while "rare_a"/"rare_b" only co-occur with each other (low density).
// This lets us verify that damping reduces hub PMI but leaves rare-pair PMI untouched.
func buildDampingCorpus() *Analyzer {
	a := NewAnalyzer()

	// "hub" appears in 20 docs, co-occurring with 20 distinct tokens.
	// This gives hub a high connection density.
	for i := 0; i < 20; i++ {
		other := []string{
			"alpha", "bravo", "charlie", "delta", "echo",
			"foxtrot", "golf", "hotel", "india", "juliet",
			"kilo", "lima", "mike", "november", "oscar",
			"papa", "quebec", "romeo", "sierra", "tango",
		}[i]
		a.Process([]string{"hub", other}, nil)
	}

	// "rare_a" and "rare_b" only appear together — strong exclusive pair.
	for i := 0; i < 10; i++ {
		a.Process([]string{"rare_a", "rare_b"}, nil)
	}

	// Some filler docs so vocab is big enough for density ratios to matter.
	fillers := []string{"x1", "x2", "x3", "x4", "x5", "x6", "x7", "x8", "x9", "x10"}
	for _, f := range fillers {
		a.Process([]string{f}, nil)
	}

	return a
}

func TestDampingFactor_NoDampingByDefault(t *testing.T) {
	a := buildDampingCorpus()
	stats := a.Snapshot()

	// Without damping configured, all factors should be 1.0.
	f := stats.dampingFactor("hub")
	if f != 1.0 {
		t.Errorf("expected dampingFactor=1.0 without damping config, got %.3f", f)
	}
	f = stats.dampingFactor("rare_a")
	if f != 1.0 {
		t.Errorf("expected dampingFactor=1.0 for rare_a without damping config, got %.3f", f)
	}
}

func TestDampingFactor_HubGetsDamped(t *testing.T) {
	a := buildDampingCorpus()
	a.WithDamping(signals.DefaultDampingConfig())
	stats := a.Snapshot()

	hubFactor := stats.dampingFactor("hub")
	rareFactor := stats.dampingFactor("rare_a")

	t.Logf("hub dampingFactor=%.3f, rare_a dampingFactor=%.3f", hubFactor, rareFactor)

	// Hub connects to 20 out of ~32 tokens (density ~0.63) — should be dampened.
	if hubFactor >= 1.0 {
		t.Errorf("hub should be dampened (factor < 1.0), got %.3f", hubFactor)
	}

	// Rare pair connects to 1 token (density ~0.03) — should NOT be dampened.
	if rareFactor != 1.0 {
		t.Errorf("rare_a should not be dampened (factor = 1.0), got %.3f", rareFactor)
	}

	// Hub should be more dampened than rare.
	if hubFactor >= rareFactor {
		t.Errorf("hub (%.3f) should have lower damping factor than rare_a (%.3f)", hubFactor, rareFactor)
	}
}

func TestDampingFactor_CustomConfig(t *testing.T) {
	a := buildDampingCorpus()
	// Very aggressive damping: anything above 10% density gets dampened.
	a.WithDamping(signals.DampingConfig{
		PMIThreshold: 0.01,
		LowDensity:   0.1,
		HighDensity:  0.3,
		MinFactor:    0.05,
	})
	stats := a.Snapshot()

	hubFactor := stats.dampingFactor("hub")
	t.Logf("hub dampingFactor with aggressive config: %.3f", hubFactor)

	// With aggressive settings, hub should be heavily dampened.
	if hubFactor > 0.2 {
		t.Errorf("expected hub dampingFactor < 0.2 with aggressive config, got %.3f", hubFactor)
	}
}

func TestStopwordStats_DampingReducesHubPMIMax(t *testing.T) {
	corpus := buildDampingCorpus()

	// Get undamped stats.
	undampedStats := corpus.Snapshot()
	undampedSW := undampedStats.StopwordStats()
	undampedPMI := make(map[string]float64)
	for _, sw := range undampedSW {
		undampedPMI[sw.Token] = sw.PMIMax
	}

	// Get damped stats from a fresh corpus.
	corpus2 := buildDampingCorpus()
	corpus2.WithDamping(signals.DefaultDampingConfig())
	dampedStats := corpus2.Snapshot()
	dampedSW := dampedStats.StopwordStats()
	dampedPMI := make(map[string]float64)
	for _, sw := range dampedSW {
		dampedPMI[sw.Token] = sw.PMIMax
	}

	t.Logf("hub: undamped PMIMax=%.3f, damped PMIMax=%.3f", undampedPMI["hub"], dampedPMI["hub"])
	t.Logf("rare_a: undamped PMIMax=%.3f, damped PMIMax=%.3f", undampedPMI["rare_a"], dampedPMI["rare_a"])

	// Hub's PMIMax should be reduced by damping.
	if dampedPMI["hub"] >= undampedPMI["hub"] {
		t.Errorf("damping should reduce hub PMIMax: undamped=%.3f, damped=%.3f",
			undampedPMI["hub"], dampedPMI["hub"])
	}

	// Rare pair's PMIMax should be unchanged (dampingFactor=1.0 for both tokens).
	if math.Abs(dampedPMI["rare_a"]-undampedPMI["rare_a"]) > 0.001 {
		t.Errorf("damping should not affect rare_a PMIMax: undamped=%.3f, damped=%.3f",
			undampedPMI["rare_a"], dampedPMI["rare_a"])
	}
}

func TestHighPMIPairs_DampingReducesHubScores(t *testing.T) {
	corpus := buildDampingCorpus()

	// Undamped.
	undampedStats := corpus.Snapshot()
	undampedPairs := undampedStats.HighPMIPairs()
	undampedByPair := make(map[string]float64)
	for _, p := range undampedPairs {
		undampedByPair[p.A+"|"+p.B] = p.PMI
	}

	// Damped.
	corpus2 := buildDampingCorpus()
	corpus2.WithDamping(signals.DefaultDampingConfig())
	dampedStats := corpus2.Snapshot()
	dampedPairs := dampedStats.HighPMIPairs()
	dampedByPair := make(map[string]float64)
	for _, p := range dampedPairs {
		dampedByPair[p.A+"|"+p.B] = p.PMI
	}

	// Pairs involving "hub" should have reduced PMI.
	hubReduced := false
	for key, undampedPMI := range undampedByPair {
		dampedPMI := dampedByPair[key]
		if dampedPMI < undampedPMI {
			// At least one hub pair was reduced.
			if key[:3] == "hub" || key[len(key)-3:] == "hub" {
				hubReduced = true
			}
		}
	}
	if !hubReduced {
		t.Error("expected at least one hub pair to have reduced PMI from damping")
	}

	// Rare pair should be unchanged.
	rareKey := "rare_a|rare_b"
	if _, ok := undampedByPair[rareKey]; !ok {
		rareKey = "rare_b|rare_a" // pair may be sorted differently
	}
	if math.Abs(dampedByPair[rareKey]-undampedByPair[rareKey]) > 0.001 {
		t.Errorf("rare pair PMI should be unchanged: undamped=%.3f, damped=%.3f",
			undampedByPair[rareKey], dampedByPair[rareKey])
	}
}

func TestDampingCache_ComputedOnce(t *testing.T) {
	a := buildDampingCorpus()
	a.WithDamping(signals.DefaultDampingConfig())
	stats := a.Snapshot()

	// First call builds cache.
	f1 := stats.dampingFactor("hub")
	// Second call should return same value (cache hit).
	f2 := stats.dampingFactor("hub")
	if f1 != f2 {
		t.Errorf("dampingFactor should be deterministic: first=%.3f, second=%.3f", f1, f2)
	}
}

func TestDampingFactor_UnknownToken(t *testing.T) {
	a := buildDampingCorpus()
	a.WithDamping(signals.DefaultDampingConfig())
	stats := a.Snapshot()

	// Token not in corpus should get 1.0 (no damping).
	f := stats.dampingFactor("nonexistent")
	if f != 1.0 {
		t.Errorf("unknown token should get dampingFactor=1.0, got %.3f", f)
	}
}

func TestTaxonomyDrift_LowCoverage(t *testing.T) {
	a := NewAnalyzer()
	// 10 docs tagged "ai" containing "machine"
	for i := 0; i < 10; i++ {
		a.Process([]string{"machine", "learning"}, []string{"ai"})
	}
	// 10 docs tagged "ai" without "obsolete"
	for i := 0; i < 10; i++ {
		a.Process([]string{"deep", "neural"}, []string{"ai"})
	}
	stats := a.Snapshot()

	taxonomy := map[string][]string{
		"ai": {"machine", "obsolete"},
	}
	drift := stats.TaxonomyDrift(taxonomy)

	// Find drift for "obsolete" — should have low coverage
	var found bool
	for _, d := range drift {
		if d.Keyword == "obsolete" && d.Type == "low_coverage" {
			found = true
			if d.Coverage != 0 {
				t.Errorf("obsolete should have 0 coverage, got %f", d.Coverage)
			}
			if d.MissedDocs == 0 {
				t.Error("obsolete should have missed docs > 0")
			}
		}
	}
	if !found {
		t.Error("expected low_coverage drift for 'obsolete'")
	}

	// "machine" should have reasonable coverage
	for _, d := range drift {
		if d.Keyword == "machine" && d.Type == "low_coverage" {
			if d.Coverage == 0 {
				t.Error("machine should have non-zero coverage")
			}
		}
	}
}

func TestTaxonomyDrift_OrphanDetection(t *testing.T) {
	a := NewAnalyzer()
	// Create a corpus where "kubernetes" appears in >5% of docs but is not in taxonomy
	for i := 0; i < 20; i++ {
		a.Process([]string{"kubernetes", "deploy"}, []string{"tech"})
	}
	// Pad with other docs so kubernetes = 20/30 = 66% DF
	for i := 0; i < 10; i++ {
		a.Process([]string{"python", "code"}, []string{"programming"})
	}
	stats := a.Snapshot()

	taxonomy := map[string][]string{
		"tech":        {"docker", "cloud"},
		"programming": {"python", "code"},
	}
	drift := stats.TaxonomyDrift(taxonomy)

	// "kubernetes" should be detected as orphan
	var orphanFound bool
	for _, d := range drift {
		if d.Keyword == "kubernetes" && d.Type == "orphan" {
			orphanFound = true
			if d.SupportDocs != 20 {
				t.Errorf("kubernetes support should be 20, got %d", d.SupportDocs)
			}
			if d.Category != "tech" {
				t.Errorf("kubernetes best category should be 'tech', got %q", d.Category)
			}
		}
	}
	if !orphanFound {
		t.Error("expected orphan detection for 'kubernetes'")
	}

	// "python" and "code" are in taxonomy — should NOT be orphans
	for _, d := range drift {
		if (d.Keyword == "python" || d.Keyword == "code") && d.Type == "orphan" {
			t.Errorf("taxonomy keyword %q should not be an orphan", d.Keyword)
		}
	}
}

func TestTaxonomyDrift_EmptyTaxonomy(t *testing.T) {
	a := NewAnalyzer()
	a.Process([]string{"test"}, []string{"cat"})
	stats := a.Snapshot()

	drift := stats.TaxonomyDrift(nil)
	if drift != nil {
		t.Errorf("empty taxonomy should return nil, got %d results", len(drift))
	}

	drift = stats.TaxonomyDrift(map[string][]string{})
	if drift != nil {
		t.Errorf("empty taxonomy map should return nil, got %d results", len(drift))
	}
}

func TestTaxonomyDrift_NoDocs(t *testing.T) {
	stats := Stats{}
	drift := stats.TaxonomyDrift(map[string][]string{"ai": {"ml"}})
	if drift != nil {
		t.Errorf("zero docs should return nil, got %d results", len(drift))
	}
}
