package cards

import (
	"strings"
	"unicode/utf8"
)

const maxSnippetLen = 500

// ExtractSnippet returns the first 3 sentences or maxSnippetLen characters of
// the body text, whichever is shorter. Suitable for storage and card generation.
func ExtractSnippet(body string) string {
	if body == "" {
		return ""
	}
	sentences := splitSentences(body)
	if len(sentences) > 3 {
		sentences = sentences[:3]
	}
	snippet := strings.Join(sentences, " ")

	if utf8.RuneCountInString(snippet) > maxSnippetLen {
		runes := []rune(snippet)
		snippet = string(runes[:maxSnippetLen])
		// Break at word boundary if possible
		if idx := strings.LastIndex(snippet, " "); idx > maxSnippetLen/2 {
			snippet = snippet[:idx]
		}
		snippet += "..."
	}
	return snippet
}

// splitSentences splits text into sentences on ". ", "! ", "? " boundaries.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			if i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n') {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// generateBullet extracts the most relevant sentence from a document's snippet.
// Prefers sentences containing query tokens. Falls back to first non-title
// sentence, then to the title.
func generateBullet(doc ScoredDoc, queryTokens map[string]struct{}) string {
	if doc.BodySnippet == "" {
		return doc.Title
	}
	sentences := splitSentences(doc.BodySnippet)
	if len(sentences) == 0 {
		return doc.Title
	}

	// Score each sentence by query token overlap
	bestScore := -1
	bestIdx := 0
	for i, sent := range sentences {
		score := 0
		lower := strings.ToLower(sent)
		for qt := range queryTokens {
			if strings.Contains(lower, strings.ReplaceAll(qt, "-", " ")) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	// If best sentence duplicates the title, try second-best
	if strings.EqualFold(strings.TrimSpace(sentences[bestIdx]), strings.TrimSpace(doc.Title)) && len(sentences) > 1 {
		if bestIdx == 0 {
			return sentences[1]
		}
		return sentences[bestIdx]
	}
	return sentences[bestIdx]
}
