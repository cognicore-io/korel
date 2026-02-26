package prolog

import (
	"bufio"
	"fmt"
	"strings"
	"sync"

	iprolog "github.com/ichiban/prolog"

	"github.com/cognicore/korel/pkg/korel/inference"
)

// Engine wraps ichiban/prolog with mutex protection and Korel-specific rules.
// ichiban/prolog is NOT goroutine-safe; all access is serialized via mutex.
type Engine struct {
	mu     sync.Mutex
	interp *iprolog.Interpreter
	facts  map[factKey]struct{} // dedup tracker to avoid duplicate assertions
}

type factKey struct {
	relation, subject, object string
}

// New creates a Prolog engine with built-in semantic reasoning rules loaded.
func New() (*Engine, error) {
	interp := iprolog.New(nil, nil)

	// Declare dynamic predicates so assertz works for all relation types
	dynamics := []string{
		"related_to", "category", "synonym", "is_a",
		"used_for", "part_of", "co_entity",
	}
	for _, pred := range dynamics {
		if err := interp.Exec(fmt.Sprintf(":- dynamic(%s/2).", pred)); err != nil {
			return nil, fmt.Errorf("declare dynamic %s: %w", pred, err)
		}
	}

	// Load built-in rules for semantic reasoning
	if err := interp.Exec(BuiltinRules); err != nil {
		return nil, fmt.Errorf("load builtin rules: %w", err)
	}

	return &Engine{
		interp: interp,
		facts:  make(map[factKey]struct{}, 1024),
	}, nil
}

// quoteAtom wraps a string in single quotes for Prolog.
// Embedded single quotes are doubled per ISO Prolog convention.
func quoteAtom(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}

// AddFact adds a single fact to the Prolog knowledge base.
// Duplicates are silently ignored.
func (e *Engine) AddFact(relation, subject, object string) {
	key := factKey{relation, subject, object}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.facts[key]; exists {
		return
	}
	e.facts[key] = struct{}{}

	clause := fmt.Sprintf(":- assertz(%s(%s, %s)).",
		relation, quoteAtom(subject), quoteAtom(object))
	e.interp.Exec(clause)
}

// Query checks if a relationship exists (with inference via Prolog rules).
func (e *Engine) Query(relation, subject, object string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	goal := fmt.Sprintf("%s(%s, %s).",
		relation, quoteAtom(subject), quoteAtom(object))
	sols, err := e.interp.Query(goal)
	if err != nil {
		return false
	}
	found := sols.Next()
	sols.Close()
	return found
}

// QueryAll returns all objects related to subject via the given relation.
// Includes results from both direct facts and Prolog rules.
func (e *Engine) QueryAll(relation, subject string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	goal := fmt.Sprintf("%s(%s, X).", relation, quoteAtom(subject))
	sols, err := e.interp.Query(goal)
	if err != nil {
		return nil
	}
	defer sols.Close()

	seen := make(map[string]struct{})
	var results []string
	for sols.Next() {
		var r struct {
			X string `prolog:"X"`
		}
		if err := sols.Scan(&r); err != nil {
			continue
		}
		if _, dup := seen[r.X]; !dup {
			seen[r.X] = struct{}{}
			results = append(results, r.X)
		}
	}
	return results
}

// Expand takes query tokens and returns related terms via Prolog inference.
// Uses the expand_token/2 rule which combines all reasoning modes
// (related_to, transitive, same_domain, equivalent).
func (e *Engine) Expand(tokens []string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	origSet := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		origSet[t] = struct{}{}
	}

	seen := make(map[string]struct{})
	for _, tok := range tokens {
		goal := fmt.Sprintf("expand_token(%s, X).", quoteAtom(tok))
		sols, err := e.interp.Query(goal)
		if err != nil {
			continue
		}
		for sols.Next() {
			var r struct {
				X string `prolog:"X"`
			}
			if err := sols.Scan(&r); err != nil {
				continue
			}
			if _, isOrig := origSet[r.X]; !isOrig {
				seen[r.X] = struct{}{}
			}
		}
		sols.Close()
	}

	results := make([]string, 0, len(seen))
	for tok := range seen {
		results = append(results, tok)
	}
	return results
}

// FindPath finds a chain of inferences connecting subject to object.
// Uses Go-side BFS over Prolog queries for predictable performance.
func (e *Engine) FindPath(subject, object string) []inference.Step {
	e.mu.Lock()
	defer e.mu.Unlock()

	type bfsEntry struct {
		node string
		path []inference.Step
	}

	relations := []string{"related_to", "is_a", "used_for", "category", "synonym"}
	visited := make(map[string]bool)
	queue := []bfsEntry{{node: subject}}
	visited[subject] = true

	for len(queue) > 0 && len(queue) < 1000 {
		current := queue[0]
		queue = queue[1:]

		if len(current.path) > 5 {
			continue // max depth
		}

		for _, rel := range relations {
			goal := fmt.Sprintf("%s(%s, X).", rel, quoteAtom(current.node))
			sols, err := e.interp.Query(goal)
			if err != nil {
				continue
			}

			for sols.Next() {
				var r struct {
					X string `prolog:"X"`
				}
				if err := sols.Scan(&r); err != nil {
					continue
				}

				step := inference.Step{
					Relation: rel,
					From:     current.node,
					To:       r.X,
					Depth:    len(current.path),
					Rule:     fmt.Sprintf("%s(%s, %s)", rel, current.node, r.X),
				}
				newPath := make([]inference.Step, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(current.path)] = step

				if r.X == object {
					sols.Close()
					return newPath
				}

				if !visited[r.X] {
					visited[r.X] = true
					queue = append(queue, bfsEntry{node: r.X, path: newPath})
				}
			}
			sols.Close()
		}
	}

	return nil
}

// LoadRules loads facts and rules from a text string.
// Supports two formats:
//   - Prolog clauses (rules with :-)
//   - Simple facts: relation(subject, object)
func (e *Engine) LoadRules(rules string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	scanner := bufio.NewScanner(strings.NewReader(rules))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "%") {
			continue
		}

		// If it's already valid Prolog (has a period at end), exec directly
		if strings.HasSuffix(line, ".") {
			if err := e.interp.Exec(line); err != nil {
				return fmt.Errorf("line %d: %w", lineNum, err)
			}
			continue
		}

		// Otherwise, parse simple format: relation(subject, object)
		fact, err := parseSimpleFact(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		clause := fmt.Sprintf(":- assertz(%s(%s, %s)).",
			fact.Relation, quoteAtom(fact.Subject), quoteAtom(fact.Object))
		if err := e.interp.Exec(clause); err != nil {
			return fmt.Errorf("line %d: assert %w", lineNum, err)
		}
	}
	return scanner.Err()
}

// Explain generates a human-readable explanation of an inference.
func (e *Engine) Explain(relation, subject, object string) string {
	if e.Query(relation, subject, object) {
		path := e.FindPath(subject, object)
		if len(path) == 0 {
			return fmt.Sprintf("%s(%s, %s) is directly known", relation, subject, object)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Inference chain for %s(%s, %s):\n", relation, subject, object))
		for i, step := range path {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.Rule))
		}
		return b.String()
	}
	return fmt.Sprintf("Cannot prove %s(%s, %s)", relation, subject, object)
}

// parseSimpleFact parses "relation(subject, object)" format (no trailing period).
func parseSimpleFact(line string) (inference.Fact, error) {
	openParen := strings.Index(line, "(")
	if openParen == -1 {
		return inference.Fact{}, fmt.Errorf("missing '(': %s", line)
	}

	relation := strings.TrimSpace(line[:openParen])

	closeParen := strings.LastIndex(line, ")")
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
