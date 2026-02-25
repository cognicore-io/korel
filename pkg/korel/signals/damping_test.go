package signals

import (
	"math"
	"testing"
)

type mockDensityProvider struct {
	counts map[string]int
	vocab  int
}

func (m *mockDensityProvider) NeighborCount(token string, _ float64) int {
	return m.counts[token]
}

func (m *mockDensityProvider) VocabSize() int {
	return m.vocab
}

func TestComputeDamping_LowDensityNoDamping(t *testing.T) {
	provider := &mockDensityProvider{
		counts: map[string]int{"solar": 10},
		vocab:  1000,
	}

	d := ComputeDamping("solar", provider, DefaultDampingConfig())

	if d.DampingFactor != 1.0 {
		t.Errorf("expected factor 1.0 for low density (ratio=%f), got %f",
			d.DensityRatio, d.DampingFactor)
	}
}

func TestComputeDamping_HighDensityMaxDamping(t *testing.T) {
	provider := &mockDensityProvider{
		counts: map[string]int{"the": 700},
		vocab:  1000,
	}

	cfg := DefaultDampingConfig()
	d := ComputeDamping("the", provider, cfg)

	if d.DampingFactor != cfg.MinFactor {
		t.Errorf("expected factor %f for high density (ratio=%f), got %f",
			cfg.MinFactor, d.DensityRatio, d.DampingFactor)
	}
}

func TestComputeDamping_MidDensityPartialDamping(t *testing.T) {
	// 450/1000 = 0.45, which is between LowDensity=0.3 and HighDensity=0.6
	provider := &mockDensityProvider{
		counts: map[string]int{"data": 450},
		vocab:  1000,
	}

	d := ComputeDamping("data", provider, DefaultDampingConfig())

	if d.DampingFactor >= 1.0 || d.DampingFactor <= 0.1 {
		t.Errorf("expected partial damping for mid density, got factor %f (ratio=%f)",
			d.DampingFactor, d.DensityRatio)
	}
}

func TestComputeDamping_NilProvider(t *testing.T) {
	d := ComputeDamping("any", nil, DefaultDampingConfig())
	if d.DampingFactor != 1.0 {
		t.Errorf("expected factor 1.0 for nil provider, got %f", d.DampingFactor)
	}
}

func TestComputeDamping_ZeroVocab(t *testing.T) {
	provider := &mockDensityProvider{
		counts: map[string]int{"x": 5},
		vocab:  0,
	}
	d := ComputeDamping("x", provider, DefaultDampingConfig())
	if d.DensityRatio != 0 {
		t.Errorf("expected ratio 0 for zero vocab, got %f", d.DensityRatio)
	}
	if d.DampingFactor != 1.0 {
		t.Errorf("expected factor 1.0 for zero vocab, got %f", d.DampingFactor)
	}
}

func TestComputeDamping_SmoothstepMonotonic(t *testing.T) {
	provider := &mockDensityProvider{vocab: 1000}
	cfg := DefaultDampingConfig()

	// Factor should monotonically decrease as neighbor count increases
	var prev float64 = 1.1
	for count := 0; count <= 1000; count += 50 {
		provider.counts = map[string]int{"tok": count}
		d := ComputeDamping("tok", provider, cfg)
		if d.DampingFactor > prev+1e-9 {
			t.Errorf("non-monotonic: count=%d factor=%f > prev=%f", count, d.DampingFactor, prev)
		}
		prev = d.DampingFactor
	}
}

func TestComputeDamping_SmoothstepMidpoint(t *testing.T) {
	// At midpoint of transition (ratio=0.45), smoothstep should give ~0.5 blend
	provider := &mockDensityProvider{
		counts: map[string]int{"mid": 450},
		vocab:  1000,
	}
	cfg := DefaultDampingConfig()
	d := ComputeDamping("mid", provider, cfg)

	// t=0.5, smoothstep(0.5) = 0.5, factor = 1.0 - 0.5*(1.0-0.1) = 0.55
	expected := 1.0 - 0.5*(1.0-cfg.MinFactor)
	if math.Abs(d.DampingFactor-expected) > 0.01 {
		t.Errorf("at midpoint expected factor ~%f, got %f", expected, d.DampingFactor)
	}
}

func TestComputeDampingBatch(t *testing.T) {
	provider := &mockDensityProvider{
		counts: map[string]int{
			"rare":   5,
			"common": 500,
			"hub":    900,
		},
		vocab: 1000,
	}

	results := ComputeDampingBatch([]string{"rare", "common", "hub"}, provider, DefaultDampingConfig())
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Rare should have highest factor, hub should have lowest
	if results[0].DampingFactor < results[1].DampingFactor {
		t.Errorf("rare (%f) should have higher factor than common (%f)",
			results[0].DampingFactor, results[1].DampingFactor)
	}
	if results[1].DampingFactor < results[2].DampingFactor {
		t.Errorf("common (%f) should have higher factor than hub (%f)",
			results[1].DampingFactor, results[2].DampingFactor)
	}
}

func TestDampedPMI(t *testing.T) {
	density := TokenDensity{DampingFactor: 0.5}
	result := DampedPMI(2.0, density)
	if result != 1.0 {
		t.Errorf("expected 2.0 * 0.5 = 1.0, got %f", result)
	}
}

func TestDampedPMIPair(t *testing.T) {
	a := TokenDensity{DampingFactor: 0.5}
	b := TokenDensity{DampingFactor: 0.5}
	result := DampedPMIPair(2.0, a, b)
	// sqrt(0.5 * 0.5) = 0.5, so 2.0 * 0.5 = 1.0
	if result != 1.0 {
		t.Errorf("expected 1.0, got %f", result)
	}

	// Asymmetric: one hub, one specific
	c := TokenDensity{DampingFactor: 1.0}
	d := TokenDensity{DampingFactor: 0.1}
	result = DampedPMIPair(2.0, c, d)
	expected := 2.0 * math.Sqrt(1.0*0.1)
	if math.Abs(result-expected) > 0.001 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestDampedPMI_NoDamping(t *testing.T) {
	density := TokenDensity{DampingFactor: 1.0}
	result := DampedPMI(1.5, density)
	if result != 1.5 {
		t.Errorf("expected no damping (1.5), got %f", result)
	}
}
