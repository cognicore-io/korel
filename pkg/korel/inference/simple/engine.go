package simple

import (
	"bufio"
	"fmt"
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

// Expand takes query tokens and returns expanded terms via inference
func (e *Engine) Expand(tokens []string) []string {
	expanded := make(map[string]bool)

	// Add original tokens
	for _, tok := range tokens {
		expanded[tok] = true
	}

	// Expand via is_a (find all supertypes and subtypes)
	for _, tok := range tokens {
		// Supertypes (what is this a type of?)
		supers := e.QueryAll("is_a", tok)
		for _, s := range supers {
			expanded[s] = true
		}

		// Subtypes (what things are types of this?)
		for subj, objs := range e.facts["is_a"] {
			for _, obj := range objs {
				if obj == tok {
					expanded[subj] = true
				}
			}
		}

		// Related concepts
		related := e.QueryAll("related_to", tok)
		for _, r := range related {
			expanded[r] = true
		}
	}

	// Convert to slice
	result := make([]string, 0, len(expanded))
	for term := range expanded {
		result = append(result, term)
	}

	return result
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
