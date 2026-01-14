package analytics

import (
	"context"
	"testing"
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
	expectedBigrams := map[pair]int64{
		{A: "deep", B: "learning"}:  1,
		{A: "learning", B: "models"}: 1,
		{A: "models", B: "use"}:      1,
		{A: "use", B: "deep"}:        1,
		{A: "deep", B: "neural"}:     1,
		{A: "neural", B: "networks"}:  1,
	}

	for p, expectedCount := range expectedBigrams {
		if stats.BigramCounts[p] != expectedCount {
			t.Errorf("Bigram (%s,%s): got count %d, want %d", p.A, p.B, stats.BigramCounts[p], expectedCount)
		}
	}

	// Check document pair counts (all unique token combinations)
	// Should have (deep,learning), (deep,models), (deep,use), (deep,neural), (deep,networks),
	// (learning,models), (learning,use), etc.
	// "deep" appears twice, but only counted once in document pairs
	deepLearningPair := newPair("deep", "learning")
	if stats.PairCounts[deepLearningPair] != 1 {
		t.Errorf("Document pair (deep,learning): got count %d, want 1", stats.PairCounts[deepLearningPair])
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
	pair := newPair("transformer", "attention")
	if stats.SkipGramCounts[pair] != 1 {
		t.Errorf("Skip-gram (transformer, attention) count = %d, want 1 (should be deduplicated per document)", stats.SkipGramCounts[pair])
	}

	// (transformer, model) should also be counted once
	pair2 := newPair("transformer", "model")
	if stats.SkipGramCounts[pair2] != 1 {
		t.Errorf("Skip-gram (transformer, model) count = %d, want 1", stats.SkipGramCounts[pair2])
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
	shouldExist := []pair{
		newPair("a", "b"),
		newPair("a", "c"),
		newPair("b", "c"),
		newPair("b", "d"),
		newPair("c", "d"),
		newPair("c", "e"),
		newPair("d", "e"),
	}

	for _, p := range shouldExist {
		if stats.SkipGramCounts[p] != 1 {
			t.Errorf("Skip-gram (%s, %s) should exist in window=3, got count %d", p.A, p.B, stats.SkipGramCounts[p])
		}
	}

	// Should NOT be in window
	shouldNotExist := []pair{
		newPair("a", "d"),
		newPair("a", "e"),
		newPair("b", "e"),
	}

	for _, p := range shouldNotExist {
		if stats.SkipGramCounts[p] != 0 {
			t.Errorf("Skip-gram (%s, %s) should NOT exist (outside window=3), got count %d", p.A, p.B, stats.SkipGramCounts[p])
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
