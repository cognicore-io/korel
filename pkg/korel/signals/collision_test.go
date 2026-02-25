package signals

import "testing"

// mockPMILookup implements PMILookup for testing.
type mockPMILookup struct {
	maxPMI  map[string]float64
	pairPMI map[[2]string]float64
}

func (m *mockPMILookup) PMIMax(token string) float64 {
	return m.maxPMI[token]
}

func (m *mockPMILookup) JointPMI(a, b string) float64 {
	if v, ok := m.pairPMI[[2]string{a, b}]; ok {
		return v
	}
	return m.pairPMI[[2]string{b, a}]
}

func newMockLookup() *mockPMILookup {
	return &mockPMILookup{
		maxPMI:  make(map[string]float64),
		pairPMI: make(map[[2]string]float64),
	}
}

func TestDetectCollisions_BasicCollision(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["perovskite"] = 2.0 // strong concept
	lookup.maxPMI["finance"] = 1.8    // strong concept
	lookup.pairPMI[[2]string{"perovskite", "finance"}] = 0.05 // rarely together

	cfg := DefaultCollisionConfig()
	collisions := DetectCollisions([]string{"perovskite", "finance"}, lookup, cfg)

	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	c := collisions[0]
	if c.A != "perovskite" || c.B != "finance" {
		t.Errorf("unexpected pair: %s, %s", c.A, c.B)
	}
	if c.Surprise <= 0 {
		t.Errorf("expected positive surprise, got %f", c.Surprise)
	}
}

func TestDetectCollisions_NoCollisionWhenJointPMIHigh(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["neural"] = 2.0
	lookup.maxPMI["network"] = 2.0
	lookup.pairPMI[[2]string{"neural", "network"}] = 2.5 // always together

	cfg := DefaultCollisionConfig()
	collisions := DetectCollisions([]string{"neural", "network"}, lookup, cfg)

	if len(collisions) != 0 {
		t.Fatalf("expected 0 collisions for common pair, got %d", len(collisions))
	}
}

func TestDetectCollisions_WeakTokensIgnored(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["the"] = 0.01  // weak (stopword-like)
	lookup.maxPMI["solar"] = 1.5 // strong

	cfg := DefaultCollisionConfig()
	collisions := DetectCollisions([]string{"the", "solar"}, lookup, cfg)

	if len(collisions) != 0 {
		t.Fatalf("expected 0 collisions when one token is weak, got %d", len(collisions))
	}
}

func TestDetectCollisions_SingleToken(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["solar"] = 1.5

	cfg := DefaultCollisionConfig()
	collisions := DetectCollisions([]string{"solar"}, lookup, cfg)

	if len(collisions) != 0 {
		t.Fatalf("expected 0 collisions for single token, got %d", len(collisions))
	}
}

func TestDetectCollisions_NilLookup(t *testing.T) {
	collisions := DetectCollisions([]string{"a", "b"}, nil, DefaultCollisionConfig())
	if collisions != nil {
		t.Fatal("expected nil for nil lookup")
	}
}

func TestDetectCollisions_SortedBySurprise(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["a"] = 2.0
	lookup.maxPMI["b"] = 2.0
	lookup.maxPMI["c"] = 2.0
	lookup.pairPMI[[2]string{"a", "b"}] = 0.1  // some joint PMI
	lookup.pairPMI[[2]string{"a", "c"}] = 0.0  // zero joint PMI = more surprising
	lookup.pairPMI[[2]string{"b", "c"}] = 0.05 // low joint PMI

	cfg := DefaultCollisionConfig()
	collisions := DetectCollisions([]string{"a", "b", "c"}, lookup, cfg)

	if len(collisions) < 2 {
		t.Fatalf("expected at least 2 collisions, got %d", len(collisions))
	}

	for i := 1; i < len(collisions); i++ {
		if collisions[i].Surprise > collisions[i-1].Surprise {
			t.Errorf("collisions not sorted: [%d].Surprise=%f > [%d].Surprise=%f",
				i, collisions[i].Surprise, i-1, collisions[i-1].Surprise)
		}
	}
}

func TestDetectCollisions_EmptyTokens(t *testing.T) {
	lookup := newMockLookup()
	collisions := DetectCollisions(nil, lookup, DefaultCollisionConfig())
	if collisions != nil {
		t.Fatal("expected nil for empty tokens")
	}
}

func TestDetectCollisions_MinSurpriseFilter(t *testing.T) {
	lookup := newMockLookup()
	lookup.maxPMI["a"] = 0.6 // just above MinStrength
	lookup.maxPMI["b"] = 0.6
	lookup.pairPMI[[2]string{"a", "b"}] = 0.25 // just below MaxJointPMI

	// With high MinSurprise, this marginal case should be filtered
	cfg := CollisionConfig{
		MinStrength: 0.5,
		MaxJointPMI: 0.3,
		MinSurprise: 1.0, // very high bar
	}
	collisions := DetectCollisions([]string{"a", "b"}, lookup, cfg)
	if len(collisions) != 0 {
		t.Fatalf("expected 0 collisions with high MinSurprise, got %d", len(collisions))
	}
}
