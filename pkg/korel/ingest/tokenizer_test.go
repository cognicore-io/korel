package ingest

import (
	"strings"
	"testing"

	"github.com/cognicore/korel/pkg/korel/lexicon"
)

func TestTokenizerBasic(t *testing.T) {
	stopwords := []string{"the", "a", "and", "of"}
	tokenizer := NewTokenizer(stopwords)

	text := "The quick brown fox jumps over the lazy dog"
	tokens := tokenizer.Tokenize(text)

	// "the" should be filtered out
	for _, tok := range tokens {
		if tok == "the" {
			t.Error("Stopword 'the' should be filtered")
		}
	}

	// Should contain content words
	expected := []string{"quick", "brown", "fox", "jumps", "over", "lazy", "dog"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}
}

func TestTokenizerHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "machine-learning and deep-learning"
	tokens := tokenizer.Tokenize(text)

	// Should preserve hyphens
	hasHyphen := false
	for _, tok := range tokens {
		if tok == "machine-learning" || tok == "deep-learning" {
			hasHyphen = true
			break
		}
	}

	if !hasHyphen {
		t.Error("Hyphenated words should be preserved")
	}
}

func TestTokenizerCaseNormalization(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "BERT GPT-4 Transformer"
	tokens := tokenizer.Tokenize(text)

	// Should lowercase
	for _, tok := range tokens {
		if tok != strings.ToLower(tok) {
			t.Errorf("Token %s should be lowercased", tok)
		}
	}
}

func TestAddRemoveStopword(t *testing.T) {
	tokenizer := NewTokenizer([]string{"the"})

	// Initially "the" is filtered
	tokens := tokenizer.Tokenize("the cat")
	if len(tokens) != 1 || tokens[0] != "cat" {
		t.Error("Should filter 'the'")
	}

	// Remove from stoplist
	tokenizer.RemoveStopword("the")
	tokens = tokenizer.Tokenize("the cat")
	if len(tokens) != 2 {
		t.Error("'the' should not be filtered after removal")
	}

	// Add back
	tokenizer.AddStopword("the")
	tokens = tokenizer.Tokenize("the cat")
	if len(tokens) != 1 || tokens[0] != "cat" {
		t.Error("Should filter 'the' after re-adding")
	}
}

func TestTokenizerEmptyInput(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	tokens := tokenizer.Tokenize("")
	if len(tokens) != 0 {
		t.Error("Empty input should produce empty output")
	}
}

func TestTokenizerSpecialCharacters(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "hello@world.com test#tag 123"
	tokens := tokenizer.Tokenize(text)

	// Should keep alphanumeric and hyphens, filter special chars
	// Expected: ["helloworld", "com", "test", "tag", "123"]
	for _, tok := range tokens {
		if !isAlphaNumericOrHyphen(tok) {
			t.Errorf("Token %s contains invalid characters", tok)
		}
	}
}

func isAlphaNumericOrHyphen(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

// Edge case tests

func TestTokenizerVeryLongWord(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Create a very long word (200 characters)
	longWord := strings.Repeat("verylongword", 20)
	text := "normal " + longWord + " text"
	tokens := tokenizer.Tokenize(text)

	// Should handle long words without panic
	if len(tokens) != 3 {
		t.Errorf("Expected 3 tokens, got %d", len(tokens))
	}
}

func TestTokenizerUnicodeCharacters(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Unicode letters should be handled
	text := "café résumé naïve"
	tokens := tokenizer.Tokenize(text)

	// Should tokenize unicode letters
	if len(tokens) == 0 {
		t.Error("Unicode text should produce tokens")
	}
}

func TestTokenizerMultipleConsecutiveHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "test---word and--another"
	tokens := tokenizer.Tokenize(text)

	// Should handle multiple hyphens (now also supports numbers)
	for _, tok := range tokens {
		if !isAlphaNumericOrHyphen(tok) {
			t.Errorf("Token %s contains invalid characters", tok)
		}
	}
}

func TestTokenizerWhitespaceOnly(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "   \t\n\r   "
	tokens := tokenizer.Tokenize(text)

	if len(tokens) != 0 {
		t.Errorf("Whitespace-only input should produce 0 tokens, got %d", len(tokens))
	}
}

func TestTokenizerSingleCharacterFiltering(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Single characters should be filtered (len > 1 requirement)
	text := "a b c machine learning"
	tokens := tokenizer.Tokenize(text)

	// Should not contain single-character tokens
	for _, tok := range tokens {
		if len(tok) == 1 {
			t.Errorf("Single character token should be filtered: %s", tok)
		}
	}

	// Should contain multi-character tokens
	hasMultiChar := false
	for _, tok := range tokens {
		if tok == "machine-learning" || tok == "machine" {
			hasMultiChar = true
		}
	}
	if !hasMultiChar {
		t.Error("Should contain multi-character tokens")
	}
}

func TestTokenizerOnlyHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "normal text"
	tokens := tokenizer.Tokenize(text)

	// Should tokenize normal text
	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(tokens))
	}

	// Hyphen-only sequences are currently accepted as tokens
	// This documents the current behavior
	text2 := "word-with-hyphens"
	tokens2 := tokenizer.Tokenize(text2)
	if len(tokens2) == 0 {
		t.Error("Hyphenated words should be tokenized")
	}
}

func TestTokenizerMixedPunctuation(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "hello! world? test... end."
	tokens := tokenizer.Tokenize(text)

	// Punctuation should be removed
	expected := []string{"hello", "world", "test", "end"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
}

func TestTokenizerNumbersFiltered(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "machine learning 2023 gpt-4 utf-8"
	tokens := tokenizer.Tokenize(text)

	// Pure-numeric tokens are filtered (low semantic value).
	// Mixed tokens like "gpt-4", "utf-8" are kept.
	want := []string{"machine", "learning", "gpt-4", "utf-8"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerStopwordCaseInsensitive(t *testing.T) {
	tokenizer := NewTokenizer([]string{"THE", "A"})

	text := "The cat and the dog"
	tokens := tokenizer.Tokenize(text)

	// Stopwords should be filtered regardless of case
	for _, tok := range tokens {
		if tok == "the" || tok == "a" {
			t.Errorf("Stopword should be filtered regardless of case: %s", tok)
		}
	}
}

func TestTokenizerEmptyStopwordList(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	text := "the quick brown fox"
	tokens := tokenizer.Tokenize(text)

	// With no stopwords, should keep all words (except single chars)
	if len(tokens) != 4 {
		t.Errorf("Expected 4 tokens with no stopwords, got %d", len(tokens))
	}
}

func TestTokenizerDuplicateStopwords(t *testing.T) {
	// Adding duplicate stopwords should not cause issues
	tokenizer := NewTokenizer([]string{"the", "the", "a", "the"})

	text := "the cat"
	tokens := tokenizer.Tokenize(text)

	if len(tokens) != 1 || tokens[0] != "cat" {
		t.Errorf("Duplicate stopwords should work correctly, got %v", tokens)
	}
}

// Tests for real-world issues discovered during bootstrap

func TestTokenizerLeadingHyphenFromURLSplit(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// When URLs are stripped, we can get leading hyphens like "-patch-would-ms-ext"
	text := "text -patch-would-ms-ext linux"
	tokens := tokenizer.Tokenize(text)

	// Should strip leading hyphen and keep the rest
	want := []string{"text", "patch-would-ms-ext", "linux"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerTrailingHyphenFromURLSplit(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// When part of URL is stripped: "gpt-model" where "model" gets removed → "gpt-"
	text := "gpt- model test-"
	tokens := tokenizer.Tokenize(text)

	// Should strip trailing hyphen
	want := []string{"gpt", "model", "test"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerBothLeadingAndTrailingHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// From taxonomy: "-emulator-assembler-game-vhdl-"
	text := "-emulator-assembler-game-vhdl- text"
	tokens := tokenizer.Tokenize(text)

	// Should strip both leading and trailing hyphens
	want := []string{"emulator-assembler-game-vhdl", "text"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerHyphenOnlyToken(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Edge case: just hyphens
	text := "normal - -- --- ---- text"
	tokens := tokenizer.Tokenize(text)

	// Should filter out hyphen-only tokens (too short after strip)
	want := []string{"normal", "text"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerValidHyphenatedTerms(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Real technical terms that should be preserved
	text := "state-of-the-art machine-learning deep-learning x-ray utf-8"
	tokens := tokenizer.Tokenize(text)

	want := []string{"state-of-the-art", "machine-learning", "deep-learning", "x-ray", "utf-8"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerMultipleConsecutiveHyphensNormalized(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Multiple consecutive hyphens should be treated as separators
	text := "test--double normal---triple"
	tokens := tokenizer.Tokenize(text)

	// test--double becomes "test--double" which should be normalized
	// This test documents current behavior; may want to normalize to single hyphen
	for _, tok := range tokens {
		if strings.Contains(tok, "--") || strings.Contains(tok, "---") {
			t.Errorf("Token %q contains consecutive hyphens, should be normalized", tok)
		}
	}
}

func TestTokenizerShortTokensWithHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Short tokens with hyphens after stripping
	text := "a- -b --c d--"
	tokens := tokenizer.Tokenize(text)

	// After stripping leading/trailing hyphens:
	// "a-" → "a" (1 char, filtered)
	// "-b" → "b" (1 char, filtered)
	// "--c" → "c" (1 char, filtered)
	// "d--" → "d" (1 char, filtered)
	want := []string{}
	if len(tokens) != len(want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerMixedValidInvalidHyphens(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Mix of valid hyphenated terms and URL fragments
	text := "machine-learning -garbage end- normal-text"
	tokens := tokenizer.Tokenize(text)

	// All should have leading/trailing hyphens stripped
	want := []string{"machine-learning", "garbage", "end", "normal-text"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerAuthorNamePairs(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Author names from arXiv abstracts
	// These should tokenize normally; filtering happens at PMI pair level
	text := "Karol Szpunar Abigail Lin"
	tokens := tokenizer.Tokenize(text)

	want := []string{"karol", "szpunar", "abigail", "lin"}
	if !equalTokens(tokens, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", text, tokens, want)
	}
}

func TestTokenizerRealWorldURLFragments(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Real examples from HN corpus after URL stripping
	text := "million-to-one ruby-ffi-gc-bug-hash-becomes-string -fitzgerald-wreck-diving"
	tokens := tokenizer.Tokenize(text)

	// Should preserve valid hyphenated compounds, strip leading hyphens
	wantContains := []string{"million-to-one", "ruby-ffi-gc-bug-hash-becomes-string", "fitzgerald-wreck-diving"}
	for _, want := range wantContains {
		found := false
		for _, got := range tokens {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected token %q not found in %v", want, tokens)
		}
	}
}

// Helper function for comparing token lists
func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============================================================================
// Lexicon Integration Tests
// ============================================================================

func TestTokenizerLexiconNormalization(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	// Create lexicon with synonym groups
	lex := lexicon.New()
	lex.AddSynonymGroup("game", []string{"games", "gaming", "gamer", "gamers"})
	lex.AddSynonymGroup("analyze", []string{"analysis", "analytical", "analyzer"})

	tokenizer.SetLexicon(lex)

	// Test text with variants
	text := "Gaming analysis shows that gamers prefer analytical tools"
	tokens := tokenizer.Tokenize(text)

	// "gaming" should be normalized to "game"
	// "analysis" should be normalized to "analyze"
	// "gamers" should be normalized to "game"
	// "analytical" should be normalized to "analyze"

	foundGame := false
	foundAnalyze := false
	for _, tok := range tokens {
		if tok == "game" {
			foundGame = true
		}
		if tok == "analyze" {
			foundAnalyze = true
		}
		// Should NOT have the original variants
		if tok == "gaming" || tok == "gamers" {
			t.Errorf("Token %q should have been normalized to 'game'", tok)
		}
		if tok == "analysis" || tok == "analytical" {
			t.Errorf("Token %q should have been normalized to 'analyze'", tok)
		}
	}

	if !foundGame {
		t.Error("Expected normalized token 'game' not found")
	}
	if !foundAnalyze {
		t.Error("Expected normalized token 'analyze' not found")
	}
}

func TestTokenizerLexiconWithoutLexicon(t *testing.T) {
	tokenizer := NewTokenizer([]string{})
	// No lexicon set - should work normally

	text := "Gaming analysis for gamers"
	tokens := tokenizer.Tokenize(text)

	// Without lexicon, should preserve original forms
	expected := map[string]bool{
		"gaming":   false,
		"analysis": false,
		"gamers":   false,
	}

	for _, tok := range tokens {
		if _, ok := expected[tok]; ok {
			expected[tok] = true
		}
	}

	for word, found := range expected {
		if !found {
			t.Errorf("Without lexicon, expected original word %q in tokens", word)
		}
	}
}

func TestTokenizerLexiconMultiToken(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	lex := lexicon.New()
	lex.AddSynonymGroup("ml", []string{"machine learning", "machine-learning", "ML"})

	tokenizer.SetLexicon(lex)

	// Note: Multi-token phrases like "machine learning" are challenging for the tokenizer
	// because it tokenizes word-by-word. This test documents current behavior.
	text := "machine learning and ML"
	tokens := tokenizer.Tokenize(text)

	// Each word is tokenized separately:
	// "machine" → "machine" (no synonym match as single word)
	// "learning" → "learning" (no synonym match as single word)
	// "ML" → "ml" (matches, normalizes to "ml")

	// This shows that multi-token normalization needs phrase detection first
	foundML := false
	for _, tok := range tokens {
		if tok == "ml" {
			foundML = true
		}
	}

	if !foundML {
		t.Error("Expected 'ML' to normalize to 'ml'")
	}
}

func TestTokenizerLexiconWithStopwords(t *testing.T) {
	stopwords := []string{"the", "a", "for"}
	tokenizer := NewTokenizer(stopwords)

	lex := lexicon.New()
	lex.AddSynonymGroup("game", []string{"games", "gaming"})

	tokenizer.SetLexicon(lex)

	text := "The gaming industry for games"
	tokens := tokenizer.Tokenize(text)

	// "the" and "for" should be filtered as stopwords
	// "gaming" and "games" should both normalize to "game"
	// Result should have "game" (possibly appearing once due to normalization)

	for _, tok := range tokens {
		if tok == "the" || tok == "for" {
			t.Errorf("Stopword %q should be filtered", tok)
		}
		if tok == "gaming" || tok == "games" {
			t.Errorf("Variant %q should be normalized to 'game'", tok)
		}
	}

	// Should contain "game" and "industry"
	expectedTokens := map[string]bool{"game": false, "industry": false}
	for _, tok := range tokens {
		if _, ok := expectedTokens[tok]; ok {
			expectedTokens[tok] = true
		}
	}

	for word, found := range expectedTokens {
		if !found {
			t.Errorf("Expected token %q not found", word)
		}
	}
}

func TestTokenizerLexiconCaseInsensitive(t *testing.T) {
	tokenizer := NewTokenizer([]string{})

	lex := lexicon.New()
	lex.AddSynonymGroup("ai", []string{"AI", "artificial intelligence"})

	tokenizer.SetLexicon(lex)

	text := "AI and Ai are the same"
	tokens := tokenizer.Tokenize(text)

	// Both "AI" and "Ai" should normalize to "ai"
	for _, tok := range tokens {
		if tok == "AI" || tok == "Ai" {
			t.Errorf("Token %q should be normalized to lowercase 'ai'", tok)
		}
	}

	// Should have "ai" (deduplicated)
	foundAI := false
	for _, tok := range tokens {
		if tok == "ai" {
			foundAI = true
		}
	}

	if !foundAI {
		t.Error("Expected normalized 'ai' token")
	}
}
