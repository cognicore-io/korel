package korel

import (
	"strings"

	"github.com/cognicore/korel/pkg/korel/store"
)

// RewriteResult contains the rewritten query and metadata about applied rules.
type RewriteResult struct {
	Original  string
	Rewritten string
	Applied   []RewriteRule
}

// RewriteRule describes a single transformation applied to the query.
type RewriteRule struct {
	Name string // "synonym", "canonical"
	From string
	To   string
}

// Rewriter transforms queries before processing.
// Uses the dictionary to expand known phrases to their canonical forms.
type Rewriter struct {
	dict store.DictView
}

// NewRewriter creates a Rewriter backed by the given dictionary view.
// Returns nil if dict is nil.
func NewRewriter(dict store.DictView) *Rewriter {
	if dict == nil {
		return nil
	}
	return &Rewriter{dict: dict}
}

// Rewrite applies dictionary-based synonym expansion and canonicalization.
// Each token in the query is looked up in the dictionary; if a canonical
// form exists and differs, it replaces the original token.
func (r *Rewriter) Rewrite(query string) RewriteResult {
	result := RewriteResult{Original: query}
	if r.dict == nil {
		result.Rewritten = query
		return result
	}

	words := strings.Fields(query)
	changed := false

	for i, word := range words {
		lower := strings.ToLower(word)
		canonical, _, ok := r.dict.Lookup(lower)
		if ok && canonical != "" && canonical != lower {
			result.Applied = append(result.Applied, RewriteRule{
				Name: "canonical",
				From: lower,
				To:   canonical,
			})
			words[i] = canonical
			changed = true
		}
	}

	if changed {
		result.Rewritten = strings.Join(words, " ")
	} else {
		result.Rewritten = query
	}
	return result
}
