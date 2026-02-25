package signals

import (
	"sort"
	"testing"
)

type mockNeighborProvider struct {
	neighbors map[string][]string
}

func (m *mockNeighborProvider) TopNeighbors(token string, k int) []string {
	n := m.neighbors[token]
	if len(n) > k {
		return n[:k]
	}
	return n
}

func TestComputePredictionError_PerfectPrediction(t *testing.T) {
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"solar": {"photovoltaic", "panel", "energy"},
		},
	}

	// Results contain exactly what PMI predicted
	pe := ComputePredictionError(
		[]string{"solar"},
		[]string{"solar", "photovoltaic", "panel", "energy"},
		provider,
		DefaultPredictionConfig(),
	)

	if pe.Score != 0.0 {
		t.Errorf("expected score 0 for perfect prediction, got %f", pe.Score)
	}
	if len(pe.OnlyPredicted) != 0 {
		t.Errorf("expected no only-predicted, got %v", pe.OnlyPredicted)
	}
	if len(pe.OnlyActual) != 0 {
		t.Errorf("expected no only-actual, got %v", pe.OnlyActual)
	}
}

func TestComputePredictionError_CompleteSurprise(t *testing.T) {
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"solar": {"photovoltaic", "panel", "energy"},
		},
	}

	// Results contain nothing PMI predicted
	pe := ComputePredictionError(
		[]string{"solar"},
		[]string{"solar", "blockchain", "crypto", "defi"},
		provider,
		DefaultPredictionConfig(),
	)

	if pe.Score != 1.0 {
		t.Errorf("expected score 1.0 for complete surprise, got %f", pe.Score)
	}
	if len(pe.Overlap) != 0 {
		t.Errorf("expected no overlap, got %v", pe.Overlap)
	}

	sort.Strings(pe.OnlyPredicted)
	if len(pe.OnlyPredicted) != 3 {
		t.Errorf("expected 3 only-predicted, got %d", len(pe.OnlyPredicted))
	}

	sort.Strings(pe.OnlyActual)
	if len(pe.OnlyActual) != 3 {
		t.Errorf("expected 3 only-actual, got %d", len(pe.OnlyActual))
	}
}

func TestComputePredictionError_PartialMatch(t *testing.T) {
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"bert": {"transformer", "nlp", "attention", "encoder"},
		},
	}

	// Results have some predicted and some unexpected
	pe := ComputePredictionError(
		[]string{"bert"},
		[]string{"bert", "transformer", "attention", "vision", "image"},
		provider,
		DefaultPredictionConfig(),
	)

	if pe.Score <= 0.0 || pe.Score >= 1.0 {
		t.Errorf("expected partial score between 0 and 1, got %f", pe.Score)
	}

	sort.Strings(pe.Overlap)
	expected := []string{"attention", "transformer"}
	if len(pe.Overlap) != len(expected) {
		t.Fatalf("expected overlap %v, got %v", expected, pe.Overlap)
	}
	for i := range expected {
		if pe.Overlap[i] != expected[i] {
			t.Errorf("overlap[%d] = %s, want %s", i, pe.Overlap[i], expected[i])
		}
	}
}

func TestComputePredictionError_NilProvider(t *testing.T) {
	pe := ComputePredictionError([]string{"a"}, []string{"b"}, nil, DefaultPredictionConfig())
	if pe.Score != 0 {
		t.Errorf("expected 0 for nil provider, got %f", pe.Score)
	}
}

func TestComputePredictionError_EmptyQuery(t *testing.T) {
	provider := &mockNeighborProvider{}
	pe := ComputePredictionError(nil, []string{"b"}, provider, DefaultPredictionConfig())
	if pe.Score != 0 {
		t.Errorf("expected 0 for empty query, got %f", pe.Score)
	}
}

func TestComputePredictionError_QueryTokensExcluded(t *testing.T) {
	// Query tokens should not appear in predicted/actual comparison
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"solar": {"solar", "panel"}, // "solar" in neighbors should be excluded
		},
	}

	pe := ComputePredictionError(
		[]string{"solar"},
		[]string{"solar", "panel"},
		provider,
		DefaultPredictionConfig(),
	)

	// "solar" should be excluded from both predicted and actual
	// Only "panel" should be in predicted and actual
	if pe.Score != 0.0 {
		t.Errorf("expected score 0 (panel predicted and found), got %f", pe.Score)
	}
}

func TestComputePredictionError_MultipleQueryTokens(t *testing.T) {
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"solar":  {"panel", "energy", "photovoltaic"},
			"policy": {"government", "regulation", "tariff"},
		},
	}

	pe := ComputePredictionError(
		[]string{"solar", "policy"},
		[]string{"solar", "policy", "panel", "tariff", "subsidy"},
		provider,
		DefaultPredictionConfig(),
	)

	// predicted: panel, energy, photovoltaic, government, regulation, tariff (6)
	// actual: panel, tariff, subsidy (3, excluding query tokens)
	// overlap: panel, tariff (2)
	// union: 6 + 3 - 2 = 7
	// score: 1 - 2/7 = 0.714...
	if pe.Score < 0.7 || pe.Score > 0.75 {
		t.Errorf("expected score ~0.714, got %f", pe.Score)
	}
}

func TestComputePredictionError_EmptyResults(t *testing.T) {
	provider := &mockNeighborProvider{
		neighbors: map[string][]string{
			"solar": {"panel", "energy"},
		},
	}

	pe := ComputePredictionError(
		[]string{"solar"},
		nil, // no results
		provider,
		DefaultPredictionConfig(),
	)

	if pe.Score != 1.0 {
		t.Errorf("expected score 1.0 for empty results with predictions, got %f", pe.Score)
	}
}
