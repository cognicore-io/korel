package prolog

import (
	"fmt"
	"sync"
	"testing"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return e
}

func TestPrologBasicFacts(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "gpt", "transformer")
	e.AddFact("is_a", "transformer", "neural-network")

	if !e.Query("is_a", "bert", "transformer") {
		t.Error("Expected bert is_a transformer")
	}

	if !e.Query("is_a", "gpt", "transformer") {
		t.Error("Expected gpt is_a transformer")
	}

	// Negative: should NOT find non-existent fact
	if e.Query("is_a", "bert", "cnn") {
		t.Error("bert should not be is_a cnn")
	}
}

func TestPrologQueryAll(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("is_a", "bert", "transformer")
	e.AddFact("is_a", "bert", "model")
	e.AddFact("is_a", "gpt", "transformer")

	results := e.QueryAll("is_a", "bert")

	expected := map[string]bool{"transformer": true, "model": true}
	if len(results) != len(expected) {
		t.Errorf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}
	for _, r := range results {
		if !expected[r] {
			t.Errorf("Unexpected result: %s", r)
		}
	}
}

func TestPrologExpand(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("related_to", "bert", "transformer")
	e.AddFact("related_to", "transformer", "attention")
	e.AddFact("related_to", "transformer", "nlp")

	expanded := e.Expand([]string{"bert"})

	// Should include at least "transformer" (direct) and "attention"/"nlp" (transitive)
	asSet := toSet(expanded)
	if !asSet["transformer"] {
		t.Errorf("Expected expanded to contain transformer, got %v", expanded)
	}

	// Original token should NOT be in expansion
	if asSet["bert"] {
		t.Error("Original token 'bert' should not be in expansion results")
	}
}

func TestPrologTransitiveRule(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")

	// transitive(X, Y) :- related_to(X, Z), related_to(Z, Y), X \= Y
	if !e.Query("transitive", "a", "c") {
		t.Error("Expected transitive(a, c) to hold via a→b→c")
	}

	// Direct relation should NOT be transitive (transitive requires 2 hops)
	if e.Query("transitive", "a", "b") {
		t.Error("transitive(a, b) should NOT hold — b is directly related, not transitive")
	}
}

func TestPrologSameDomainRule(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("category", "neural network", "ai")
	e.AddFact("category", "deep learning", "ai")
	e.AddFact("category", "kubernetes", "devops")

	// same_domain(X, Y) :- category(X, C), category(Y, C), X \= Y
	if !e.Query("same_domain", "neural network", "deep learning") {
		t.Error("Expected same_domain(neural network, deep learning)")
	}

	// Different domains should not be same_domain
	if e.Query("same_domain", "neural network", "kubernetes") {
		t.Error("neural network and kubernetes should NOT be same_domain")
	}
}

func TestPrologSynonymEquivalence(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("synonym", "ml", "machine learning")
	e.AddFact("synonym", "ML", "machine learning")

	// equivalent(X, Y) :- synonym(X, C), synonym(Y, C), X \= Y
	if !e.Query("equivalent", "ml", "ML") {
		t.Error("Expected equivalent(ml, ML) through shared canonical")
	}
}

func TestPrologSpacesInTokens(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("related_to", "machine learning", "neural network")
	e.AddFact("related_to", "neural network", "deep learning")

	if !e.Query("related_to", "machine learning", "neural network") {
		t.Error("Expected related_to(machine learning, neural network)")
	}

	results := e.QueryAll("related_to", "machine learning")
	if len(results) != 1 || results[0] != "neural network" {
		t.Errorf("Expected [neural network], got %v", results)
	}

	// Transitive through multi-word tokens
	if !e.Query("transitive", "machine learning", "deep learning") {
		t.Error("Expected transitive(machine learning, deep learning)")
	}
}

func TestPrologLoadRules(t *testing.T) {
	e := newTestEngine(t)

	rules := `
# Comments are skipped
is_a(bert, transformer)
is_a(gpt, transformer)
related_to(transformer, attention)
`
	if err := e.LoadRules(rules); err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	if !e.Query("is_a", "bert", "transformer") {
		t.Error("Loaded fact not found")
	}

	if !e.Query("related_to", "transformer", "attention") {
		t.Error("Loaded fact not found")
	}
}

func TestPrologFindPath(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")
	e.AddFact("related_to", "c", "d")

	path := e.FindPath("a", "d")
	if len(path) == 0 {
		t.Fatal("Expected to find path from a to d")
	}

	if path[0].From != "a" {
		t.Errorf("First step should start from 'a', got %s", path[0].From)
	}

	lastStep := path[len(path)-1]
	if lastStep.To != "d" {
		t.Errorf("Last step should end at 'd', got %s", lastStep.To)
	}
}

func TestPrologExplain(t *testing.T) {
	e := newTestEngine(t)

	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "b", "c")

	explanation := e.Explain("related_to", "a", "b")
	if explanation == "" {
		t.Error("Expected non-empty explanation")
	}

	t.Logf("Explanation:\n%s", explanation)
}

func TestPrologDuplicateFacts(t *testing.T) {
	e := newTestEngine(t)

	// Adding the same fact twice should not cause duplicates in results
	e.AddFact("related_to", "a", "b")
	e.AddFact("related_to", "a", "b")

	results := e.QueryAll("related_to", "a")
	if len(results) != 1 {
		t.Errorf("Expected 1 result (deduped), got %d: %v", len(results), results)
	}
}

func TestPrologConcurrentAccess(t *testing.T) {
	e := newTestEngine(t)

	// Pre-load some facts
	for i := 0; i < 50; i++ {
		e.AddFact("related_to", "hub", fmt.Sprintf("node_%d", i))
	}

	// Concurrent reads should not panic or deadlock
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.QueryAll("related_to", "hub")
			e.Expand([]string{"hub"})
			e.Query("related_to", "hub", "node_0")
		}()
	}
	wg.Wait()
}

func TestPrologExpandCombinesRules(t *testing.T) {
	e := newTestEngine(t)

	// Setup: related_to + category + synonym
	e.AddFact("related_to", "neural network", "deep learning")
	e.AddFact("category", "neural network", "ai")
	e.AddFact("category", "transformer", "ai")
	e.AddFact("synonym", "nn", "neural network")
	e.AddFact("synonym", "ann", "neural network")

	expanded := e.Expand([]string{"neural network"})
	asSet := toSet(expanded)

	// related_to: deep learning
	if !asSet["deep learning"] {
		t.Errorf("Expected 'deep learning' via related_to, got %v", expanded)
	}

	// same_domain: transformer (shares 'ai' category)
	if !asSet["transformer"] {
		t.Errorf("Expected 'transformer' via same_domain, got %v", expanded)
	}

	t.Logf("Expanded: %v", expanded)
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// TestPrologSameDomainExpansion proves Prolog finds concepts that share a
// taxonomy category even without ANY direct related_to edge between them.
// The simple engine cannot do this — it only walks explicit edges.
func TestPrologSameDomainExpansion(t *testing.T) {
	e := newTestEngine(t)

	// Setup: kubernetes and docker share "devops" category.
	// NO related_to edge between them.
	e.AddFact("category", "kubernetes", "devops")
	e.AddFact("category", "docker", "devops")
	e.AddFact("category", "terraform", "devops")
	e.AddFact("category", "pytorch", "ai")
	e.AddFact("category", "tensorflow", "ai")

	// Expand "kubernetes" — should find docker and terraform via same_domain
	expanded := e.Expand([]string{"kubernetes"})
	asSet := toSet(expanded)

	if !asSet["docker"] {
		t.Errorf("Expected 'docker' via same_domain(kubernetes, docker), got %v", expanded)
	}
	if !asSet["terraform"] {
		t.Errorf("Expected 'terraform' via same_domain(kubernetes, terraform), got %v", expanded)
	}

	// Should NOT find pytorch — different category, no edge
	if asSet["pytorch"] {
		t.Errorf("'pytorch' is in 'ai' category, should not appear in devops expansion, got %v", expanded)
	}

	// Category names themselves should NOT appear in expansion
	// (expand_token returns concepts, not categories)
	if asSet["devops"] {
		t.Errorf("Category name 'devops' should not appear in expansion, got %v", expanded)
	}

	t.Logf("Expanded 'kubernetes': %v", expanded)
}

// TestPrologSynonymExpansion proves Prolog finds equivalent terms through
// shared canonical forms, without needing explicit related_to edges.
func TestPrologSynonymExpansion(t *testing.T) {
	e := newTestEngine(t)

	// "ml" and "ML" are both synonyms of "machine learning"
	e.AddFact("synonym", "ml", "machine learning")
	e.AddFact("synonym", "ai/ml", "machine learning")
	// No related_to edges at all

	expanded := e.Expand([]string{"ml"})
	asSet := toSet(expanded)

	// Should find "ai/ml" through equivalent(ml, ai/ml) — both synonyms of "machine learning"
	if !asSet["ai/ml"] {
		t.Errorf("Expected 'ai/ml' via equivalent(ml, ai/ml), got %v", expanded)
	}

	t.Logf("Expanded 'ml': %v", expanded)
}

// TestPrologCrossDomainBridge proves Prolog finds bridge concepts that
// connect different categories.
func TestPrologCrossDomainBridge(t *testing.T) {
	e := newTestEngine(t)

	// mlops bridges ai and devops
	e.AddFact("category", "mlops", "ai")
	e.AddFact("category", "mlops", "devops")
	e.AddFact("category", "kubernetes", "devops")
	e.AddFact("related_to", "kubernetes", "mlops")

	// bridge(kubernetes, mlops) should hold:
	// category(kubernetes, devops), category(mlops, ai), devops \= ai, related_to(kubernetes, mlops)
	if !e.Query("bridge", "kubernetes", "mlops") {
		t.Error("Expected bridge(kubernetes, mlops) — crosses devops→ai via related_to")
	}

	t.Log("Bridge detection working")
}

// TestPrologComposedExpansion proves that composed rules find concepts
// that are reachable only through chaining: related_to → same_domain.
// The simple engine cannot do this at its default BFS depth.
func TestPrologComposedExpansion(t *testing.T) {
	e := newTestEngine(t)

	// "query term" is related to "concept a".
	// "concept a" shares category "cat" with "concept b" and "concept c".
	// There is NO direct edge from "query term" to "concept b".
	e.AddFact("related_to", "query term", "concept a")
	e.AddFact("category", "concept a", "cat")
	e.AddFact("category", "concept b", "cat")
	e.AddFact("category", "concept c", "cat")

	expanded := e.Expand([]string{"query term"})
	asSet := toSet(expanded)

	// Direct neighbor
	if !asSet["concept a"] {
		t.Errorf("Expected 'concept a' via related_to, got %v", expanded)
	}

	// Composed: related_to(query term, concept a), same_domain(concept a, concept b)
	if !asSet["concept b"] {
		t.Errorf("Expected 'concept b' via composed related_to→same_domain, got %v", expanded)
	}
	if !asSet["concept c"] {
		t.Errorf("Expected 'concept c' via composed related_to→same_domain, got %v", expanded)
	}

	// Category name should NOT appear
	if asSet["cat"] {
		t.Errorf("Category name 'cat' should not appear in expansion, got %v", expanded)
	}

	t.Logf("Expanded 'query term': %v", expanded)
}

// ensure fmt is used
var _ = fmt.Sprintf
