package simple

import (
	"fmt"
	"testing"
)

func TestBasicFacts(t *testing.T) {
	e := New()

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "gpt", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")

	if !e.Query("is_a", "bert", "transformer") {
		t.Error("Expected bert is_a transformer")
	}

	if !e.Query("is_a", "bert", "neural-network") {
		t.Error("Expected transitive: bert is_a neural-network")
	}
}

func TestQueryAll(t *testing.T) {
	e := New()

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")
	e.AddFact("is_a", "neural-network", "model")

	results := e.QueryAll("is_a", "bert")

	expected := map[string]bool{
		"transformer":     true,
		"neural-network":  true,
		"model":           true,
	}

	if len(results) != len(expected) {
		t.Errorf("Expected %d results, got %d", len(expected), len(results))
	}

	for _, r := range results {
		if !expected[r] {
			t.Errorf("Unexpected result: %s", r)
		}
	}
}

func TestExpand(t *testing.T) {
	e := New()

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "gpt", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")
	e.AddFact("related_to", "transformer", "attention-mechanism")

	expanded := e.Expand([]string{"bert"})

	// Expand returns only expanded terms (not originals).
	// Should include: transformer (1 hop), neural-network (2 hops), gpt (2 hops via reverse is_a)
	expectedContains := []string{"transformer", "neural-network"}

	for _, exp := range expectedContains {
		found := false
		for _, r := range expanded {
			if r == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected expanded to contain: %s (got %v)", exp, expanded)
		}
	}

	// Original token should NOT be in expansion.
	for _, r := range expanded {
		if r == "bert" {
			t.Error("Original token 'bert' should not be in expansion results")
		}
	}
}

func TestLoadRules(t *testing.T) {
	e := New()

	rules := `
# AI taxonomy
is_a(bert, transformer)
is_a(gpt, transformer)
is_a(transformer, neural-network)

# Relations
used_for(transformer, nlp)
related_to(bert, attention-mechanism)
`

	if err := e.LoadRules(rules); err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	if !e.Query("is_a", "bert", "neural-network") {
		t.Error("Transitive inference failed after loading rules")
	}

	if !e.Query("used_for", "transformer", "nlp") {
		t.Error("Direct fact not loaded correctly")
	}
}

func TestFindPath(t *testing.T) {
	e := New()

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")
	e.AddFact("is_a", "neural-network", "model")

	path := e.FindPath("bert", "model")

	if len(path) == 0 {
		t.Fatal("Expected to find path from bert to model")
	}

	// Path should be: bert → transformer → neural-network → model
	if len(path) != 3 {
		t.Errorf("Expected path length 3, got %d", len(path))
	}

	if path[0].From != "bert" || path[0].To != "transformer" {
		t.Errorf("First step wrong: %s → %s", path[0].From, path[0].To)
	}
}

func TestExpandWithDepth_Chain(t *testing.T) {
	e := New()
	// Linear chain: a → b → c → d
	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")
	e.AddFact("related_to", "c", "d")

	// Depth 1: should reach b only (from a's direct neighbors).
	got := e.ExpandWithDepth([]string{"a"}, 1, 50)
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("depth=1: expected [b], got %v", got)
	}

	// Depth 2: should reach b and c.
	got = e.ExpandWithDepth([]string{"a"}, 2, 50)
	asSet := toSet(got)
	if !asSet["b"] || !asSet["c"] {
		t.Errorf("depth=2: expected {b, c}, got %v", got)
	}
	if asSet["d"] {
		t.Errorf("depth=2: should NOT reach d, got %v", got)
	}

	// Depth 3: should reach b, c, d.
	got = e.ExpandWithDepth([]string{"a"}, 3, 50)
	asSet = toSet(got)
	if !asSet["b"] || !asSet["c"] || !asSet["d"] {
		t.Errorf("depth=3: expected {b, c, d}, got %v", got)
	}
}

func TestExpandWithDepth_CycleDetection(t *testing.T) {
	e := New()
	// Cycle: a → b → c → a
	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")
	e.AddFact("related_to", "c", "a")

	// Should not infinite loop; should return b and c.
	got := e.ExpandWithDepth([]string{"a"}, 5, 50)
	asSet := toSet(got)
	if !asSet["b"] || !asSet["c"] {
		t.Errorf("expected {b, c}, got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(got), got)
	}
}

func TestExpandWithDepth_ConfidenceDecay(t *testing.T) {
	e := New()
	// Chain: a → b → c → d → e → f
	// Confidence: b=0.7, c=0.49, d=0.343, e=0.240, f=0.168
	// With minConfidence=0.3, d should be the last included.
	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")
	e.AddFact("related_to", "c", "d")
	e.AddFact("related_to", "d", "e")
	e.AddFact("related_to", "e", "f")

	got := e.ExpandWithDepth([]string{"a"}, 10, 50) // large depth, but confidence should prune
	asSet := toSet(got)

	if !asSet["b"] || !asSet["c"] || !asSet["d"] {
		t.Errorf("expected b, c, d to be included, got %v", got)
	}
	if asSet["e"] || asSet["f"] {
		t.Errorf("expected e, f to be pruned by confidence decay, got %v", got)
	}
}

func TestExpandWithDepth_MaxResults(t *testing.T) {
	e := New()
	// Hub: a connected to many nodes.
	for i := 0; i < 20; i++ {
		e.AddFact("related_to", "a", fmt.Sprintf("n%02d", i))
	}

	got := e.ExpandWithDepth([]string{"a"}, 1, 5)
	if len(got) != 5 {
		t.Errorf("expected 5 results with maxResults=5, got %d: %v", len(got), got)
	}
}

func TestExpandWithDepth_ReverseEdges(t *testing.T) {
	e := New()
	// b is_a a (so from a's perspective, b is a reverse neighbor).
	e.AddFact("is_a", "b", "a")

	got := e.ExpandWithDepth([]string{"a"}, 1, 50)
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("expected [b] via reverse edge, got %v", got)
	}
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func TestExplain(t *testing.T) {
	e := New()

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")

	explanation := e.Explain("is_a", "bert", "neural-network")

	if explanation == "" {
		t.Error("Expected non-empty explanation")
	}

	t.Logf("Explanation:\n%s", explanation)
}
