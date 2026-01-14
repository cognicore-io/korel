package lexicon

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Lexicon stores domain-specific vocabulary mappings:
// - Synonyms: different words with same meaning (car ↔ automobile)
// - Variants: inflections/forms (analyze ↔ analysis ↔ analytical)
// - Acronyms: abbreviations (anova ↔ analysis of variance)
// - Multi-tokens: phrases mapped to canonical forms (machine learning → ml)
//
// Design principles:
// - Bidirectional: can normalize to canonical OR expand canonical to all variants
// - Explainable: track which synonym/variant was used for matching
// - Corpus-specific: built from bootstrap PMI pairs + manual curation
type Lexicon struct {
	// canonical -> all variants (including canonical itself)
	// Example: "game" -> ["game", "games", "gaming", "gamer"]
	synonyms map[string][]string

	// variant -> canonical
	// Example: "gaming" -> "game", "gamer" -> "game"
	reverseIndex map[string]string

	// c-tokens: contextual co-occurrence within window
	// Example: "transformer" -> ["attention", "bert", "gpt"] (tokens often appearing nearby)
	ctokens map[string][]CToken
}

// CToken represents a contextually related token with co-occurrence strength.
type CToken struct {
	Token   string  // The related token
	PMI     float64 // Pointwise mutual information (semantic strength)
	Support int64   // Number of documents where they co-occur within window
}

// New creates an empty lexicon.
func New() *Lexicon {
	return &Lexicon{
		synonyms:     make(map[string][]string),
		reverseIndex: make(map[string]string),
		ctokens:      make(map[string][]CToken),
	}
}

// LoadFromYAML loads synonym mappings from a YAML file.
//
// Expected format:
//   synonyms:
//     - canonical: game
//       variants: [games, gaming, gamer]
//     - canonical: analyze
//       variants: [analysis, analytical, analyzer]
//     - canonical: ml
//       variants: [machine learning, machine-learning]
//
// Notes:
// - Multi-token variants are supported (e.g., "machine learning")
// - Case-insensitive: all tokens normalized to lowercase
// - Canonical is included in its own variant list
func LoadFromYAML(path string) (*Lexicon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Synonyms []struct {
			Canonical string   `yaml:"canonical"`
			Variants  []string `yaml:"variants"`
		} `yaml:"synonyms"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	lex := New()
	for _, entry := range config.Synonyms {
		canonical := strings.ToLower(entry.Canonical)
		variants := make([]string, 0, len(entry.Variants)+1)
		variants = append(variants, canonical) // Include canonical itself

		for _, v := range entry.Variants {
			normalized := strings.ToLower(v)
			if normalized != canonical {
				variants = append(variants, normalized)
			}
		}

		lex.AddSynonymGroup(canonical, variants)
	}

	return lex, nil
}

// AddSynonymGroup adds a synonym group with a canonical form and its variants.
// The canonical form is always included as the first entry in the variants list.
// If the group already exists, old reverse index entries are cleaned up first.
func (l *Lexicon) AddSynonymGroup(canonical string, variants []string) {
	canonical = strings.ToLower(canonical)

	// Clean up old reverse index entries if this canonical already exists
	if oldVariants, exists := l.synonyms[canonical]; exists {
		for _, oldV := range oldVariants {
			delete(l.reverseIndex, oldV)
		}
	}

	// Ensure canonical is first in the list and deduplicate
	normalized := make([]string, 0, len(variants)+1)
	seen := make(map[string]bool)

	// Add canonical first
	normalized = append(normalized, canonical)
	seen[canonical] = true

	// Add other variants
	for _, v := range variants {
		v = strings.ToLower(v)
		if !seen[v] {
			normalized = append(normalized, v)
			seen[v] = true
		}
	}

	l.synonyms[canonical] = normalized

	// Build reverse index: each variant points to canonical
	for _, v := range normalized {
		l.reverseIndex[v] = canonical
	}
}

// Normalize returns the canonical form of a token.
// If the token is not in the lexicon, returns the token itself.
//
// Examples:
//   - Normalize("gaming") -> "game"
//   - Normalize("unknown") -> "unknown"
func (l *Lexicon) Normalize(token string) string {
	token = strings.ToLower(token)
	if canonical, ok := l.reverseIndex[token]; ok {
		return canonical
	}
	return token
}

// Variants returns all known variants of a token (including the canonical form).
// If the token is not in the lexicon, returns a slice containing only the token itself.
//
// Examples:
//   - Variants("game") -> ["game", "games", "gaming", "gamer"]
//   - Variants("gaming") -> ["game", "games", "gaming", "gamer"]
//   - Variants("unknown") -> ["unknown"]
func (l *Lexicon) Variants(token string) []string {
	token = strings.ToLower(token)

	// Check if token is canonical
	if variants, ok := l.synonyms[token]; ok {
		return variants
	}

	// Check if token is a variant
	if canonical, ok := l.reverseIndex[token]; ok {
		if variants, ok := l.synonyms[canonical]; ok {
			return variants
		}
	}

	// Not in lexicon - return token itself
	return []string{token}
}

// HasSynonyms returns true if the token has synonyms/variants in the lexicon.
func (l *Lexicon) HasSynonyms(token string) bool {
	token = strings.ToLower(token)
	_, exists := l.reverseIndex[token]
	return exists
}

// AddCToken adds a contextual token relationship.
// This records that tokenA and tokenB frequently co-occur within a window.
// If the same related token already exists, keeps the entry with highest PMI
// (or highest support if PMI is equal), preventing duplicates.
func (l *Lexicon) AddCToken(token string, related CToken) {
	token = strings.ToLower(token)
	related.Token = strings.ToLower(related.Token)

	existing := l.ctokens[token]

	// Check if this related token already exists
	found := false
	for i, ct := range existing {
		if ct.Token == related.Token {
			// Keep the stronger relationship (higher PMI, or higher support if PMI equal)
			if related.PMI > ct.PMI || (related.PMI == ct.PMI && related.Support > ct.Support) {
				existing[i] = related
			}
			found = true
			break
		}
	}

	if !found {
		existing = append(existing, related)
	}

	l.ctokens[token] = existing
}

// GetCTokens returns contextually related tokens for query expansion.
// Returns empty slice if no c-tokens are registered.
func (l *Lexicon) GetCTokens(token string) []CToken {
	token = strings.ToLower(token)
	return l.ctokens[token]
}

// Stats returns statistics about the lexicon contents.
func (l *Lexicon) Stats() LexiconStats {
	totalVariants := 0
	for _, variants := range l.synonyms {
		totalVariants += len(variants)
	}

	totalCTokens := 0
	for _, ctoks := range l.ctokens {
		totalCTokens += len(ctoks)
	}

	return LexiconStats{
		SynonymGroups: len(l.synonyms),
		TotalVariants: totalVariants,
		CTokenEntries: len(l.ctokens),
		TotalCTokens:  totalCTokens,
	}
}

// LexiconStats holds statistics about lexicon contents.
type LexiconStats struct {
	SynonymGroups int // Number of canonical forms (synonym groups)
	TotalVariants int // Total number of variants across all groups
	CTokenEntries int // Number of tokens with c-token relationships
	TotalCTokens  int // Total number of c-token relationships
}
