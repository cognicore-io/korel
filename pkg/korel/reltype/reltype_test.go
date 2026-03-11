package reltype

import (
	"testing"
)

func TestClassify_Synonym(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinNeighborOverlap = 0.3
	cfg.MaxCooccurrenceRatio = 0.5
	cfg.MinConfidence = 0.2
	c := NewClassifier(cfg)

	// Two tokens with very similar neighbor sets but low co-occurrence
	// (classic synonym signal: writers pick one or the other)
	shared := []string{"model", "training", "neural", "architecture", "layer",
		"performance", "dataset", "learning", "deep", "network"}

	neighborsA := make([]Neighbor, 0, 15)
	for _, s := range shared {
		neighborsA = append(neighborsA, Neighbor{Token: s, PMI: 0.8})
	}
	neighborsA = append(neighborsA, Neighbor{Token: "unique_a1", PMI: 0.3})
	neighborsA = append(neighborsA, Neighbor{Token: "unique_a2", PMI: 0.2})

	neighborsB := make([]Neighbor, 0, 15)
	for _, s := range shared {
		neighborsB = append(neighborsB, Neighbor{Token: s, PMI: 0.7})
	}
	neighborsB = append(neighborsB, Neighbor{Token: "unique_b1", PMI: 0.3})
	neighborsB = append(neighborsB, Neighbor{Token: "unique_b2", PMI: 0.2})

	result := c.Classify(neighborsA, neighborsB,
		100, 95,  // similar DF
		5,        // very low co-occurrence
		10000,    // corpus size
	)

	if result.Type != SameAs {
		t.Errorf("expected SameAs, got %s (confidence: %.3f)", result.Type, result.Confidence)
	}
	if result.Confidence < 0.2 {
		t.Errorf("expected confidence >= 0.2, got %.3f", result.Confidence)
	}
}

func TestClassify_NotSynonym_HighCooccurrence(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClassifier(cfg)

	// Same high overlap, but they co-occur a lot → not synonyms
	shared := []string{"model", "training", "neural", "architecture", "layer",
		"performance", "dataset", "learning", "deep", "network"}

	neighbors := make([]Neighbor, len(shared))
	for i, s := range shared {
		neighbors[i] = Neighbor{Token: s, PMI: 0.8}
	}

	result := c.Classify(neighbors, neighbors,
		100, 100,
		80, // high co-occurrence
		10000,
	)

	if result.Type == SameAs {
		t.Errorf("should not classify as SameAs when co-occurrence is high")
	}
}

func TestClassify_BroaderNarrower(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InclusionThreshold = 0.6
	cfg.MinConfidence = 0.3
	c := NewClassifier(cfg)

	// A (specific term) has neighbors that are a subset of B's (general term)
	// B has many more contexts than A
	neighborsA := []Neighbor{
		{Token: "shared1", PMI: 0.9},
		{Token: "shared2", PMI: 0.8},
		{Token: "shared3", PMI: 0.7},
		{Token: "shared4", PMI: 0.6},
		{Token: "shared5", PMI: 0.5},
		{Token: "specific1", PMI: 0.4},
	}

	neighborsB := []Neighbor{
		{Token: "shared1", PMI: 0.9},
		{Token: "shared2", PMI: 0.8},
		{Token: "shared3", PMI: 0.7},
		{Token: "shared4", PMI: 0.6},
		{Token: "shared5", PMI: 0.5},
		{Token: "general1", PMI: 0.4},
		{Token: "general2", PMI: 0.3},
		{Token: "general3", PMI: 0.2},
		{Token: "general4", PMI: 0.1},
		{Token: "general5", PMI: 0.05},
	}

	result := c.Classify(neighborsA, neighborsB,
		30,    // A appears less often (specific)
		200,   // B appears more often (general)
		20,    // some co-occurrence
		10000,
	)

	if result.Type != Broader {
		t.Errorf("expected Broader (B is more general), got %s (confidence: %.3f)", result.Type, result.Confidence)
	}
}

func TestClassify_Related_Default(t *testing.T) {
	c := NewClassifier(DefaultConfig())

	// Two tokens with different neighbor sets → plain related_to
	neighborsA := []Neighbor{
		{Token: "alpha", PMI: 0.9},
		{Token: "beta", PMI: 0.8},
		{Token: "gamma", PMI: 0.7},
	}
	neighborsB := []Neighbor{
		{Token: "delta", PMI: 0.9},
		{Token: "epsilon", PMI: 0.8},
		{Token: "zeta", PMI: 0.7},
	}

	result := c.Classify(neighborsA, neighborsB, 100, 100, 10, 10000)

	if result.Type != Related {
		t.Errorf("expected Related, got %s", result.Type)
	}
}

func TestNeighborJaccard(t *testing.T) {
	a := []Neighbor{{Token: "x", PMI: 1}, {Token: "y", PMI: 0.9}, {Token: "z", PMI: 0.8}}
	b := []Neighbor{{Token: "x", PMI: 1}, {Token: "y", PMI: 0.9}, {Token: "w", PMI: 0.8}}

	j := neighborJaccard(a, b, 20)
	// intersection=2, union=4, jaccard=0.5
	if j < 0.49 || j > 0.51 {
		t.Errorf("expected jaccard ~0.5, got %.3f", j)
	}
}

func TestWeedsPrecision(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClassifier(cfg)

	// A has 3 neighbors, all in B's set → WP(A→B) = 1.0
	a := []Neighbor{{Token: "x", PMI: 1}, {Token: "y", PMI: 0.9}}
	b := []Neighbor{{Token: "x", PMI: 1}, {Token: "y", PMI: 0.9}, {Token: "z", PMI: 0.8}, {Token: "w", PMI: 0.7}}

	wp := c.weedsPrecision(a, b)
	if wp < 0.99 {
		t.Errorf("expected WP ~1.0, got %.3f", wp)
	}

	// Reverse: B has 4 neighbors, only 2 in A → WP(B→A) = 0.5
	wpRev := c.weedsPrecision(b, a)
	if wpRev < 0.49 || wpRev > 0.51 {
		t.Errorf("expected WP ~0.5, got %.3f", wpRev)
	}
}
