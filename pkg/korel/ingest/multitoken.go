package ingest

import "strings"

// MultiTokenParser handles recognition of multi-word phrases
type MultiTokenParser struct {
	dict   map[string]DictEntry // phrase → canonical form
	maxLen int
}

// DictEntry represents a dictionary entry for a multi-token phrase
type DictEntry struct {
	Canonical string
	Category  string
	Variants  []string
}

// NewMultiTokenParser creates a new parser with the given dictionary
func NewMultiTokenParser(entries []DictEntry) *MultiTokenParser {
	dict := make(map[string]DictEntry)
	maxLen := 1
	for _, e := range entries {
		// Add canonical form
		canonical := strings.ToLower(e.Canonical)
		dict[canonical] = e
		if l := phraseLen(canonical); l > maxLen {
			maxLen = l
		}
		// Add all variants
		for _, v := range e.Variants {
			variant := strings.ToLower(v)
			dict[variant] = e
			if l := phraseLen(variant); l > maxLen {
				maxLen = l
			}
		}
	}
	return &MultiTokenParser{dict: dict, maxLen: maxLen}
}

// Parse applies greedy longest-match to recognize multi-token phrases
func (p *MultiTokenParser) Parse(tokens []string) []string {
	var result []string
	i := 0

	for i < len(tokens) {
		matched := ""
		matchLen := 1

		// Try matching from longest phrase to shortest (bigram)
		maxPhrase := p.maxLen
		if remaining := len(tokens) - i; maxPhrase > remaining {
			maxPhrase = remaining
		}
		for n := maxPhrase; n >= 2; n-- {
			phrase := strings.Join(tokens[i:i+n], " ")
			phraseKey := strings.ToLower(phrase)
			if entry, ok := p.dict[phraseKey]; ok {
				matched = entry.Canonical
				matchLen = n
				break
			}
		}

		if matched != "" {
			result = append(result, matched)
			i += matchLen
		} else {
			// Check if single token has a mapping (synonym normalization)
			tokenKey := strings.ToLower(tokens[i])
			if entry, ok := p.dict[tokenKey]; ok {
				result = append(result, entry.Canonical)
			} else {
				result = append(result, tokens[i])
			}
			i++
		}
	}

	return result
}

func phraseLen(phrase string) int {
	if phrase == "" {
		return 1
	}
	return len(strings.Fields(phrase))
}
