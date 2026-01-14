package ingest

import (
	"testing"
)

func TestPipelineBasic(t *testing.T) {
	tokenizer := NewTokenizer([]string{"the", "a", "and"})
	parser := NewMultiTokenParser([]DictEntry{
		{Canonical: "machine learning", Variants: []string{"ml"}, Category: "ai"},
	})
	taxonomy := NewTaxonomy()
	// Taxonomy should match "machine learning" (with space, as returned by parser)
	taxonomy.AddSector("ai", []string{"machine learning"})

	pipeline := NewPipeline(tokenizer, parser, taxonomy)

	result := pipeline.Process("The machine learning model uses ai")

	// Should have: machine learning (from multi-token), model, uses, ai
	if len(result.Tokens) < 3 {
		t.Errorf("Expected at least 3 tokens, got %d: %v", len(result.Tokens), result.Tokens)
	}

	// Should match ai category
	if len(result.Categories) != 1 || result.Categories[0] != "ai" {
		t.Errorf("Expected [ai] category, got %v", result.Categories)
	}
}

func TestPipelineEmptyText(t *testing.T) {
	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		NewMultiTokenParser([]DictEntry{}),
		NewTaxonomy(),
	)

	result := pipeline.Process("")

	if len(result.Tokens) != 0 {
		t.Errorf("Empty text should produce 0 tokens, got %d", len(result.Tokens))
	}

	if len(result.Categories) != 0 {
		t.Errorf("Empty text should produce 0 categories, got %d", len(result.Categories))
	}

	if len(result.Entities) != 0 {
		t.Errorf("Empty text should produce 0 entities, got %d", len(result.Entities))
	}
}

func TestPipelineOnlyStopwords(t *testing.T) {
	tokenizer := NewTokenizer([]string{"the", "a", "and", "of", "in"})
	pipeline := NewPipeline(
		tokenizer,
		NewMultiTokenParser([]DictEntry{}),
		NewTaxonomy(),
	)

	result := pipeline.Process("the and the of in a")

	if len(result.Tokens) != 0 {
		t.Errorf("Text with only stopwords should produce 0 tokens, got %d: %v", len(result.Tokens), result.Tokens)
	}
}

func TestPipelineOnlySpecialCharacters(t *testing.T) {
	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		NewMultiTokenParser([]DictEntry{}),
		NewTaxonomy(),
	)

	result := pipeline.Process("!@#$%^&*()_+-=[]{}|;':\",./<>?")

	if len(result.Tokens) != 0 {
		t.Errorf("Special characters should produce 0 tokens, got %d: %v", len(result.Tokens), result.Tokens)
	}
}

func TestPipelineNoTaxonomyMatch(t *testing.T) {
	taxonomy := NewTaxonomy()
	taxonomy.AddSector("finance", []string{"stock", "bond", "equity"})

	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		NewMultiTokenParser([]DictEntry{}),
		taxonomy,
	)

	result := pipeline.Process("machine learning and neural networks")

	if len(result.Categories) != 0 {
		t.Errorf("Unrelated text should match 0 categories, got %d: %v", len(result.Categories), result.Categories)
	}
}

func TestPipelineEntityExtraction(t *testing.T) {
	taxonomy := NewTaxonomy()
	taxonomy.AddEntity("company", "Tesla", []string{"tesla", "tsla"})
	taxonomy.AddEntity("company", "Apple", []string{"apple", "aapl"})

	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		NewMultiTokenParser([]DictEntry{}),
		taxonomy,
	)

	result := pipeline.Process("Tesla and Apple announced new products")

	if len(result.Entities) != 2 {
		t.Errorf("Expected 2 entities, got %d: %v", len(result.Entities), result.Entities)
	}

	// Check both entities found
	foundTesla := false
	foundApple := false
	for _, e := range result.Entities {
		if e.Type == "company" && e.Value == "Tesla" {
			foundTesla = true
		}
		if e.Type == "company" && e.Value == "Apple" {
			foundApple = true
		}
	}

	if !foundTesla {
		t.Error("Tesla entity not found")
	}
	if !foundApple {
		t.Error("Apple entity not found")
	}
}

func TestPipelineMultiTokenPriority(t *testing.T) {
	parser := NewMultiTokenParser([]DictEntry{
		{Canonical: "language model", Variants: []string{}, Category: "ai"},
		{Canonical: "large language model", Variants: []string{"llm"}, Category: "ai"},
	})

	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		parser,
		NewTaxonomy(),
	)

	result := pipeline.Process("large language model training")

	// Should match the longer phrase
	foundLLM := false
	for _, tok := range result.Tokens {
		if tok == "large language model" {
			foundLLM = true
		}
		if tok == "language model" {
			t.Error("Should match longest phrase, not shorter variant")
		}
	}

	if !foundLLM {
		t.Errorf("Should find 'large language model', got %v", result.Tokens)
	}
}

func TestPipelineCaseInsensitiveCategories(t *testing.T) {
	taxonomy := NewTaxonomy()
	taxonomy.AddSector("ai", []string{"machine-learning", "neural-network"})

	pipeline := NewPipeline(
		NewTokenizer([]string{}),
		NewMultiTokenParser([]DictEntry{}),
		taxonomy,
	)

	// Mix of cases
	result1 := pipeline.Process("Machine-Learning systems")
	result2 := pipeline.Process("NEURAL-NETWORK architecture")
	result3 := pipeline.Process("machine-learning and NEURAL-NETWORK")

	if len(result1.Categories) != 1 || result1.Categories[0] != "ai" {
		t.Errorf("Mixed case should match: %v", result1.Categories)
	}

	if len(result2.Categories) != 1 || result2.Categories[0] != "ai" {
		t.Errorf("Upper case should match: %v", result2.Categories)
	}

	if len(result3.Categories) != 1 || result3.Categories[0] != "ai" {
		t.Errorf("Multiple mixed case should match: %v", result3.Categories)
	}
}

func TestPipelineVeryLongText(t *testing.T) {
	pipeline := NewPipeline(
		NewTokenizer([]string{"the", "a"}),
		NewMultiTokenParser([]DictEntry{}),
		NewTaxonomy(),
	)

	// Generate long text
	longText := ""
	for i := 0; i < 10000; i++ {
		longText += "word "
	}

	result := pipeline.Process(longText)

	if len(result.Tokens) != 10000 {
		t.Errorf("Expected 10000 tokens, got %d", len(result.Tokens))
	}
}
