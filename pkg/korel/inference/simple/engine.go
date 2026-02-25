package simple

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"github.com/cognicore/korel/pkg/korel/inference"
)

// Engine is a minimal symbolic reasoning engine in pure Go
// Supports basic relations with transitive closure
type Engine struct {
	facts map[string]map[string][]string // relation → subject → [objects]
}

// New creates a new simple inference engine
func New() *Engine {
	return &Engine{
		facts: make(map[string]map[string][]string),
	}
}

// LoadRules loads facts from a simple rule file
// Format:
//   is_a(bert, transformer)
//   is_a(transformer, neural-network)
//   used_for(transformer, nlp)
//   # comments
func (e *Engine) LoadRules(rules string) error {
	scanner := bufio.NewScanner(strings.NewReader(rules))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse: relation(subject, object)
		fact, err := parseFact(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		e.AddFact(fact.Relation, fact.Subject, fact.Object)
	}

	return scanner.Err()
}

// AddFact adds a fact to the knowledge base
func (e *Engine) AddFact(relation, subject, object string) {
	if e.facts[relation] == nil {
		e.facts[relation] = make(map[string][]string)
	}

	// Avoid duplicates
	for _, obj := range e.facts[relation][subject] {
		if obj == object {
			return
		}
	}

	e.facts[relation][subject] = append(e.facts[relation][subject], object)
}

// Query checks if a relationship exists (with transitive closure for is_a)
func (e *Engine) Query(relation, subject, object string) bool {
	// Direct lookup
	if objs, ok := e.facts[relation][subject]; ok {
		for _, obj := range objs {
			if obj == object {
				return true
			}
		}
	}

	// Transitive closure for is_a and related_to
	if relation == "is_a" || relation == "related_to" {
		return e.queryTransitive(relation, subject, object, make(map[string]bool))
	}

	return false
}

// queryTransitive performs depth-first search for transitive relations
func (e *Engine) queryTransitive(relation, subject, object string, visited map[string]bool) bool {
	if visited[subject] {
		return false // cycle detection
	}
	visited[subject] = true

	// Check direct relationships
	if objs, ok := e.facts[relation][subject]; ok {
		for _, obj := range objs {
			if obj == object {
				return true
			}
			// Recurse
			if e.queryTransitive(relation, obj, object, visited) {
				return true
			}
		}
	}

	return false
}

// QueryAll returns all objects related to subject (including transitive)
func (e *Engine) QueryAll(relation, subject string) []string {
	results := make(map[string]bool)
	e.collectAll(relation, subject, results, make(map[string]bool))

	// Convert to slice
	out := make([]string, 0, len(results))
	for obj := range results {
		out = append(out, obj)
	}
	return out
}

func (e *Engine) collectAll(relation, subject string, results, visited map[string]bool) {
	if visited[subject] {
		return
	}
	visited[subject] = true

	if objs, ok := e.facts[relation][subject]; ok {
		for _, obj := range objs {
			results[obj] = true

			// Recurse for transitive relations
			if relation == "is_a" || relation == "related_to" {
				e.collectAll(relation, obj, results, visited)
			}
		}
	}
}

// FindPath finds an inference chain from subject to object
func (e *Engine) FindPath(subject, object string) []inference.Step {
	// Try all relation types
	for _, rel := range []string{"is_a", "used_for", "related_to"} {
		path := e.findPathDFS(rel, subject, object, []inference.Step{}, make(map[string]bool))
		if len(path) > 0 {
			return path
		}
	}
	return nil
}

func (e *Engine) findPathDFS(relation, from, to string, path []inference.Step, visited map[string]bool) []inference.Step {
	if visited[from] {
		return nil
	}
	visited[from] = true

	// Base case: direct connection
	if objs, ok := e.facts[relation][from]; ok {
		for _, obj := range objs {
			if obj == to {
				return append(path, inference.Step{
					Relation: relation,
					From:     from,
					To:       obj,
					Depth:    len(path),
					Rule:     fmt.Sprintf("%s(%s, %s)", relation, from, obj),
				})
			}
		}

		// Recursive case
		for _, obj := range objs {
			newPath := append(path, inference.Step{
				Relation: relation,
				From:     from,
				To:       obj,
				Depth:    len(path),
				Rule:     fmt.Sprintf("%s(%s, %s)", relation, from, obj),
			})
			result := e.findPathDFS(relation, obj, to, newPath, visited)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

// Expand takes query tokens and returns expanded terms via inference.
// Delegates to ExpandWithDepth with maxDepth=2 and maxResults=50.
func (e *Engine) Expand(tokens []string) []string {
	return e.ExpandWithDepth(tokens, 2, 50)
}

// ExpandWithDepth performs BFS-based multi-hop expansion over the fact graph.
// Each hop decays confidence by 0.7×; results below 0.3 are pruned.
// Returns up to maxResults expanded terms (not including the original tokens).
func (e *Engine) ExpandWithDepth(tokens []string, maxDepth, maxResults int) []string {
	type entry struct {
		token      string
		confidence float64
	}

	const decayFactor = 0.7
	const minConfidence = 0.3

	// Track best confidence per token across all hops.
	best := make(map[string]float64)
	for _, tok := range tokens {
		best[tok] = 1.0
	}

	// BFS frontier starts at depth 0 = original tokens with confidence 1.0.
	frontier := make([]entry, 0, len(tokens))
	for _, tok := range tokens {
		frontier = append(frontier, entry{tok, 1.0})
	}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		childConf := decayFactor // confidence for children of this frontier
		if depth > 0 {
			// Children inherit from their parent's confidence × decay.
			// We use per-entry confidence below.
		}
		_ = childConf

		var nextFrontier []entry
		for _, fe := range frontier {
			neighbors := e.neighbors(fe.token)
			newConf := fe.confidence * decayFactor
			if newConf < minConfidence {
				continue
			}
			for _, nb := range neighbors {
				if prev, seen := best[nb]; !seen || newConf > prev {
					best[nb] = newConf
					nextFrontier = append(nextFrontier, entry{nb, newConf})
				}
			}
		}
		frontier = nextFrontier
	}

	// Collect results excluding original tokens.
	origSet := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		origSet[tok] = struct{}{}
	}

	type scored struct {
		token      string
		confidence float64
	}
	results := make([]scored, 0, len(best))
	for tok, conf := range best {
		if _, isOrig := origSet[tok]; isOrig {
			continue
		}
		results = append(results, scored{tok, conf})
	}

	// Sort by confidence descending, then alphabetically for stability.
	sort.Slice(results, func(i, j int) bool {
		if results[i].confidence != results[j].confidence {
			return results[i].confidence > results[j].confidence
		}
		return results[i].token < results[j].token
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.token
	}
	return out
}

// neighbors returns all tokens directly connected to the given token
// across all relations (forward and reverse edges).
func (e *Engine) neighbors(token string) []string {
	seen := make(map[string]struct{})

	// Forward edges: token is subject in any relation.
	for _, subjects := range e.facts {
		if objs, ok := subjects[token]; ok {
			for _, obj := range objs {
				seen[obj] = struct{}{}
			}
		}
	}

	// Reverse edges: token is object in any relation.
	for _, subjects := range e.facts {
		for subj, objs := range subjects {
			for _, obj := range objs {
				if obj == token {
					seen[subj] = struct{}{}
				}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	return out
}

// Explain generates a human-readable explanation
func (e *Engine) Explain(relation, subject, object string) string {
	if e.Query(relation, subject, object) {
		path := e.findPathDFS(relation, subject, object, []inference.Step{}, make(map[string]bool))
		if len(path) == 0 {
			return fmt.Sprintf("%s(%s, %s) is directly known", relation, subject, object)
		}

		var explanation strings.Builder
		explanation.WriteString(fmt.Sprintf("Inference chain for %s(%s, %s):\n", relation, subject, object))
		for i, step := range path {
			explanation.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.Rule))
		}
		return explanation.String()
	}

	return fmt.Sprintf("Cannot prove %s(%s, %s)", relation, subject, object)
}

// parseFact parses "relation(subject, object)" format
func parseFact(line string) (inference.Fact, error) {
	// Find opening paren
	openParen := strings.Index(line, "(")
	if openParen == -1 {
		return inference.Fact{}, fmt.Errorf("missing '(': %s", line)
	}

	relation := strings.TrimSpace(line[:openParen])

	// Find closing paren
	closeParen := strings.Index(line, ")")
	if closeParen == -1 {
		return inference.Fact{}, fmt.Errorf("missing ')': %s", line)
	}

	args := line[openParen+1 : closeParen]
	parts := strings.Split(args, ",")
	if len(parts) != 2 {
		return inference.Fact{}, fmt.Errorf("expected 2 arguments, got %d: %s", len(parts), line)
	}

	return inference.Fact{
		Relation: relation,
		Subject:  strings.TrimSpace(parts[0]),
		Object:   strings.TrimSpace(parts[1]),
	}, nil
}
