package simple

import (
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

	// Should include: bert (original), transformer, neural-network, gpt (sibling), attention-mechanism
	expectedContains := []string{"bert", "transformer", "neural-network"}

	for _, exp := range expectedContains {
		found := false
		for _, r := range expanded {
			if r == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected expanded to contain: %s", exp)
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
