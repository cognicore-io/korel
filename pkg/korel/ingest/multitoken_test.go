package ingest

import (
	"reflect"
	"testing"
)

func TestMultiTokenBasic(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
		{Canonical: "neural network", Variants: []string{"nn"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"deep", "machine", "learning", "uses", "neural", "network"}
	result := parser.Parse(tokens)

	// Should recognize multi-tokens
	expected := []string{"deep", "machine learning", "uses", "neural network"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestMultiTokenSynonymNormalization(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	// "ml" should normalize to "machine learning"
	tokens := []string{"using", "ml", "for", "prediction"}
	result := parser.Parse(tokens)

	found := false
	for _, tok := range result {
		if tok == "machine learning" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Synonym 'ml' should normalize to 'machine learning'")
	}
}

func TestMultiTokenGreedyLongest(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "language model", Variants: []string{}, Category: "ai"},
		{Canonical: "large language model", Variants: []string{"llm"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"large", "language", "model", "training"}
	result := parser.Parse(tokens)

	// Should match longest (3-gram) not shorter (2-gram)
	if result[0] != "large language model" {
		t.Errorf("Should match longest phrase, got %v", result)
	}
}

func TestMultiTokenNoMatch(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"hello", "world"}
	result := parser.Parse(tokens)

	// Should pass through unchanged
	if !reflect.DeepEqual(tokens, result) {
		t.Errorf("Unmatched tokens should pass through: got %v", result)
	}
}

func TestMultiTokenOverlapping(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "neural network", Variants: []string{}, Category: "ai"},
		{Canonical: "convolutional neural network", Variants: []string{"cnn"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"convolutional", "neural", "network", "architecture"}
	result := parser.Parse(tokens)

	// Should match longest
	if result[0] != "convolutional neural network" {
		t.Errorf("Should match longest overlapping phrase, got %v", result)
	}
}

func TestMultiTokenEmptyDict(t *testing.T) {
	parser := NewMultiTokenParser([]DictEntry{})

	tokens := []string{"hello", "world"}
	result := parser.Parse(tokens)

	if !reflect.DeepEqual(tokens, result) {
		t.Error("Empty dictionary should pass through all tokens")
	}
}

func TestMultiTokenSingleWordVariant(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "photovoltaics", Variants: []string{"pv"}, Category: "solar"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"pv", "panels"}
	result := parser.Parse(tokens)

	if result[0] != "photovoltaics" {
		t.Errorf("Single-word variant should normalize, got %v", result)
	}
}

// Edge case tests

func TestMultiTokenVeryLongPhrase(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "very long multi word phrase canonical", Variants: []string{}, Category: "test"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"very", "long", "multi", "word", "phrase", "canonical", "text"}
	result := parser.Parse(tokens)

	// Should match the long phrase
	if result[0] != "very long multi word phrase canonical" {
		t.Errorf("Should match very long phrase, got %v", result)
	}
}

func TestMultiTokenMultipleVariants(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "artificial intelligence", Variants: []string{"ai", "a.i.", "a i"}, Category: "tech"},
	}
	parser := NewMultiTokenParser(entries)

	// Test each variant normalizes to canonical
	tests := [][]string{
		{"ai", "systems"},
		{"a.i.", "systems"},
		{"a", "i", "systems"},
	}

	for _, tokens := range tests {
		result := parser.Parse(tokens)
		found := false
		for _, tok := range result {
			if tok == "artificial intelligence" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Variant should normalize to canonical, tokens: %v, result: %v", tokens, result)
		}
	}
}

func TestMultiTokenEmptyVariants(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"machine", "learning"}
	result := parser.Parse(tokens)

	// Should still match canonical form even with no variants
	if result[0] != "machine learning" {
		t.Errorf("Should match canonical even without variants, got %v", result)
	}
}

func TestMultiTokenConsecutiveMatches(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "neural network", Variants: []string{}, Category: "ai"},
		{Canonical: "deep learning", Variants: []string{}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"neural", "network", "deep", "learning"}
	result := parser.Parse(tokens)

	// Should match both consecutive multi-tokens
	if len(result) != 2 {
		t.Errorf("Expected 2 consecutive matches, got %d: %v", len(result), result)
	}
	if result[0] != "neural network" || result[1] != "deep learning" {
		t.Errorf("Consecutive matches incorrect: %v", result)
	}
}

func TestMultiTokenPartialMatch(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning model", Variants: []string{}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	// Only partial match (missing "model")
	tokens := []string{"machine", "learning", "system"}
	result := parser.Parse(tokens)

	// Should not match partial phrase
	for _, tok := range result {
		if tok == "machine learning model" {
			t.Error("Should not match partial phrase")
		}
	}
}

func TestMultiTokenCaseSensitivity(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{"ML"}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	// Test mixed case
	tokens1 := []string{"Machine", "Learning"}
	tokens2 := []string{"ml"}
	tokens3 := []string{"ML"}

	result1 := parser.Parse(tokens1)
	result2 := parser.Parse(tokens2)
	result3 := parser.Parse(tokens3)

	// Document current behavior (case-sensitive or not)
	_ = result1
	_ = result2
	_ = result3
}

func TestMultiTokenIdenticalCanonicalAndVariant(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "ai", Variants: []string{"ai", "artificial intelligence"}, Category: "tech"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"ai", "systems"}
	result := parser.Parse(tokens)

	// Should handle canonical appearing in variants
	if result[0] != "ai" {
		t.Errorf("Should handle identical canonical and variant, got %v", result)
	}
}

func TestMultiTokenWhitespaceInCanonical(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "  machine   learning  ", Variants: []string{}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"machine", "learning"}
	result := parser.Parse(tokens)

	// Parser should handle whitespace in canonical gracefully
	// (may or may not match depending on implementation)
	_ = result
}

func TestMultiTokenDuplicateEntries(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"}, // duplicate
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"ml", "model"}
	result := parser.Parse(tokens)

	// Should handle duplicates without issue
	if result[0] != "machine learning" {
		t.Errorf("Should handle duplicate entries, got %v", result)
	}
}

func TestMultiTokenAllTokensMatch(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "a b c", Variants: []string{}, Category: "test"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"a", "b", "c"}
	result := parser.Parse(tokens)

	// Entire token sequence matches
	if len(result) != 1 || result[0] != "a b c" {
		t.Errorf("Should match entire sequence, got %v", result)
	}
}

func TestMultiTokenNestedPhrases(t *testing.T) {
	entries := []DictEntry{
		{Canonical: "deep", Variants: []string{}, Category: "ai"},
		{Canonical: "deep learning", Variants: []string{}, Category: "ai"},
		{Canonical: "deep learning model", Variants: []string{}, Category: "ai"},
	}
	parser := NewMultiTokenParser(entries)

	tokens := []string{"deep", "learning", "model"}
	result := parser.Parse(tokens)

	// Should match longest (greedy)
	if len(result) != 1 || result[0] != "deep learning model" {
		t.Errorf("Should match longest nested phrase, got %v", result)
	}
}
