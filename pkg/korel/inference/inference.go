package inference

// Engine provides symbolic reasoning capabilities
// This interface allows swapping implementations (simple Go engine, golog, SWI-Prolog bridge, etc.)
type Engine interface {
	// LoadRules loads domain knowledge from a file or string
	LoadRules(rules string) error

	// AddFact adds a single fact to the knowledge base
	// Example: AddFact("is_a", "bert", "transformer")
	AddFact(relation, subject, object string)

	// Query asks if a relationship exists (with optional inference)
	// Returns true if the relationship can be proven
	Query(relation, subject, object string) bool

	// QueryAll returns all objects related to subject via relation
	// Example: QueryAll("is_a", "bert") → ["transformer", "neural-network", "model"]
	QueryAll(relation, subject string) []string

	// FindPath finds a chain of inferences connecting subject to object
	// Returns empty slice if no path exists
	FindPath(subject, object string) []Step

	// Expand takes query tokens and returns related terms via inference
	// This is the main integration point with Korel's search pipeline
	Expand(tokens []string) []string

	// Explain generates a human-readable explanation of an inference
	Explain(relation, subject, object string) string
}

// Step represents one inference step in a proof
type Step struct {
	Relation string  // "is_a", "used_for", "related_to"
	From     string  // subject
	To       string  // object
	Depth    int     // how many hops from query
	Rule     string  // which rule was applied
}

// Fact represents a basic assertion
type Fact struct {
	Relation string
	Subject  string
	Object   string
}

// Rule represents an inference rule
// Example: "related_to(X, Y) :- is_a(X, Y)"
type Rule struct {
	Name       string
	Head       Fact      // conclusion
	Body       []Fact    // premises
	Confidence float64   // optional: rule strength (0-1)
}
