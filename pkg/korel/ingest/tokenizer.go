package ingest

import (
	"strings"
	"unicode"

	"github.com/cognicore/korel/pkg/korel/lexicon"
)

// Tokenizer handles text tokenization and normalization
type Tokenizer struct {
	stopwords map[string]struct{}
	lexicon   *lexicon.Lexicon // Optional: for synonym normalization
}

// NewTokenizer creates a new tokenizer with the given stopword list
func NewTokenizer(stopwords []string) *Tokenizer {
	stops := make(map[string]struct{}, len(stopwords))
	for _, w := range stopwords {
		stops[strings.ToLower(w)] = struct{}{}
	}
	return &Tokenizer{stopwords: stops}
}

// SetLexicon assigns a lexicon for synonym normalization.
// When set, tokens will be normalized to their canonical forms.
// Example: "gaming" → "game", "ML" → "ml"
func (t *Tokenizer) SetLexicon(lex *lexicon.Lexicon) {
	t.lexicon = lex
}

// Tokenize splits text into normalized tokens, removing stopwords.
// If a lexicon is set, tokens are normalized to their canonical forms.
func (t *Tokenizer) Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				word := t.processToken(current.String())
				if word != "" {
					tokens = append(tokens, word)
				}
				current.Reset()
			}
		}
	}

	// Don't forget the last token
	if current.Len() > 0 {
		word := t.processToken(current.String())
		if word != "" {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// processToken applies cleaning, lexicon normalization, and stopword filtering.
func (t *Tokenizer) processToken(token string) string {
	// Step 1: Clean (remove leading/trailing hyphens, etc.)
	word := t.cleanToken(token)
	if word == "" || len(word) <= 1 {
		return ""
	}

	// Step 1b: Filter pure-numeric tokens (low semantic value).
	// Mixed tokens like "gpt-4", "utf-8", "python3" are kept.
	if isNumericOnly(word) {
		return ""
	}

	// Step 2: Normalize via lexicon (if available)
	if t.lexicon != nil {
		word = t.lexicon.Normalize(word)
	}

	// Step 3: Check stopwords
	if t.isStopword(word) {
		return ""
	}

	return word
}

// cleanToken strips leading/trailing hyphens and normalizes consecutive hyphens
func (t *Tokenizer) cleanToken(token string) string {
	// Strip leading and trailing hyphens
	token = strings.Trim(token, "-")

	// Normalize multiple consecutive hyphens to single hyphen
	for strings.Contains(token, "--") {
		token = strings.ReplaceAll(token, "--", "-")
	}

	return token
}

// isNumericOnly returns true if the token contains only digits and hyphens.
func isNumericOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '-' {
			return false
		}
	}
	return true
}

func (t *Tokenizer) isStopword(word string) bool {
	_, ok := t.stopwords[word]
	return ok
}

// AddStopword adds a word to the stopword list
func (t *Tokenizer) AddStopword(word string) {
	t.stopwords[strings.ToLower(word)] = struct{}{}
}

// RemoveStopword removes a word from the stopword list
func (t *Tokenizer) RemoveStopword(word string) {
	delete(t.stopwords, strings.ToLower(word))
}
