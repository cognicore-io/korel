package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderAllEmpty(t *testing.T) {
	loader := Loader{
		StoplistPath: "",
		DictPath:     "",
		TaxonomyPath: "",
		RulesPath:    "",
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Empty loader should succeed: %v", err)
	}

	if comp.Tokenizer == nil {
		t.Error("Should have tokenizer (empty)")
	}

	if comp.Parser == nil {
		t.Error("Should have parser (empty)")
	}

	if comp.Taxonomy == nil {
		t.Error("Should have taxonomy (empty)")
	}

	if comp.Rules != "" {
		t.Errorf("Rules should be empty, got %q", comp.Rules)
	}
}

func TestLoaderNonExistentStoplist(t *testing.T) {
	loader := Loader{
		StoplistPath: "/nonexistent/stoplist.yaml",
		DictPath:     "",
		TaxonomyPath: "",
	}

	_, err := loader.Load()
	if err == nil {
		t.Error("Should error on nonexistent stoplist")
	}
}

func TestLoaderNonExistentDict(t *testing.T) {
	loader := Loader{
		StoplistPath: "",
		DictPath:     "/nonexistent/dict.txt",
		TaxonomyPath: "",
	}

	_, err := loader.Load()
	if err == nil {
		t.Error("Should error on nonexistent dict")
	}
}

func TestLoaderNonExistentTaxonomy(t *testing.T) {
	loader := Loader{
		StoplistPath: "",
		DictPath:     "",
		TaxonomyPath: "/nonexistent/taxonomy.yaml",
	}

	_, err := loader.Load()
	if err == nil {
		t.Error("Should error on nonexistent taxonomy")
	}
}

func TestLoaderValidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create stoplist
	slPath := filepath.Join(tmpDir, "stoplist.yaml")
	os.WriteFile(slPath, []byte("terms:\n  - the\n  - a\n"), 0644)

	// Create dict
	dictPath := filepath.Join(tmpDir, "dict.txt")
	os.WriteFile(dictPath, []byte("machine learning|ml|ai\n"), 0644)

	// Create taxonomy
	taxPath := filepath.Join(tmpDir, "tax.yaml")
	os.WriteFile(taxPath, []byte("sectors:\n  ai:\n    - machine-learning\n"), 0644)

	loader := Loader{
		StoplistPath: slPath,
		DictPath:     dictPath,
		TaxonomyPath: taxPath,
		RulesPath:    "",
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Valid files should load: %v", err)
	}

	// Verify components are initialized
	if comp.Tokenizer == nil {
		t.Error("Tokenizer should be initialized")
	}

	if comp.Parser == nil {
		t.Error("Parser should be initialized")
	}

	if comp.Taxonomy == nil {
		t.Error("Taxonomy should be initialized")
	}

	// Test that tokenizer has stopwords
	tokens := comp.Tokenizer.Tokenize("the machine learning system")
	hasML := false
	for _, tok := range tokens {
		if tok == "the" {
			t.Error("Stopword 'the' should be removed")
		}
		if tok == "machine" || tok == "learning" {
			hasML = true
		}
	}
	if !hasML {
		t.Error("Should have machine/learning tokens")
	}
}

func TestLoaderRulesPath(t *testing.T) {
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "rules.txt")
	os.WriteFile(rulesPath, []byte("is_a(bert, transformer)\n"), 0644)

	loader := Loader{
		StoplistPath: "",
		DictPath:     "",
		TaxonomyPath: "",
		RulesPath:    rulesPath,
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Rules path should load: %v", err)
	}

	if comp.Rules != rulesPath {
		t.Errorf("Rules should be %q, got %q", rulesPath, comp.Rules)
	}
}

func TestLoaderMalformedStoplist(t *testing.T) {
	tmpDir := t.TempDir()
	slPath := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(slPath, []byte("invalid: {yaml content\n"), 0644)

	loader := Loader{
		StoplistPath: slPath,
	}

	_, err := loader.Load()
	if err == nil {
		t.Error("Should error on malformed YAML")
	}
}

func TestLoaderMalformedTaxonomy(t *testing.T) {
	tmpDir := t.TempDir()
	taxPath := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(taxPath, []byte("sectors: [unclosed\n"), 0644)

	loader := Loader{
		TaxonomyPath: taxPath,
	}

	_, err := loader.Load()
	if err == nil {
		t.Error("Should error on malformed taxonomy")
	}
}

func TestLoaderEmptyStoplist(t *testing.T) {
	tmpDir := t.TempDir()
	slPath := filepath.Join(tmpDir, "empty.yaml")
	os.WriteFile(slPath, []byte("terms: []\n"), 0644)

	loader := Loader{
		StoplistPath: slPath,
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Empty stoplist should load: %v", err)
	}

	// Should process text normally with no stopwords
	tokens := comp.Tokenizer.Tokenize("the machine learning")
	if len(tokens) != 3 {
		t.Errorf("No stopwords should keep all tokens, got %d", len(tokens))
	}
}

func TestLoaderEmptyDict(t *testing.T) {
	tmpDir := t.TempDir()
	dictPath := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(dictPath, []byte(""), 0644)

	loader := Loader{
		DictPath: dictPath,
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Empty dict should load: %v", err)
	}

	// Parser should pass through tokens unchanged
	result := comp.Parser.Parse([]string{"machine", "learning"})
	if len(result) != 2 {
		t.Errorf("Empty parser should preserve tokens, got %d", len(result))
	}
}

func TestLoaderComplexTaxonomy(t *testing.T) {
	tmpDir := t.TempDir()
	taxPath := filepath.Join(tmpDir, "complex.yaml")
	content := `
sectors:
  ai:
    - machine-learning
    - neural-network
  finance:
    - stocks
    - bonds
events:
  release:
    - launched
    - released
regions:
  europe:
    - berlin
    - paris
entities:
  tickers:
    TSLA:
      - tesla
      - tsla
`
	os.WriteFile(taxPath, []byte(content), 0644)

	loader := Loader{
		TaxonomyPath: taxPath,
	}

	comp, err := loader.Load()
	if err != nil {
		t.Fatalf("Complex taxonomy should load: %v", err)
	}

	// Test sector matching
	cats := comp.Taxonomy.AssignCategories([]string{"machine-learning", "stocks"})
	if len(cats) != 2 {
		t.Errorf("Should match 2 sectors, got %d: %v", len(cats), cats)
	}

	// Test entity extraction
	entities := comp.Taxonomy.ExtractEntities("Tesla announced new model")
	if len(entities) != 1 {
		t.Errorf("Should find 1 entity, got %d", len(entities))
	}
}
