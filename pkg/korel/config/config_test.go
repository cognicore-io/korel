package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStoplist(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stoplist.yaml")

	content := `terms:
  - the
  - a
  - and
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sl, err := LoadStoplist(path)
	if err != nil {
		t.Fatalf("Failed to load stoplist: %v", err)
	}

	if len(sl.Terms) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(sl.Terms))
	}

	expected := map[string]bool{"the": true, "a": true, "and": true}
	for _, term := range sl.Terms {
		if !expected[term] {
			t.Errorf("Unexpected term: %s", term)
		}
	}
}

func TestLoadTaxonomy(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "taxonomy.yaml")

	content := `sectors:
  ai:
    - machine-learning
    - neural-network
  web:
    - html
    - css

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
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tax, err := LoadTaxonomy(path)
	if err != nil {
		t.Fatalf("Failed to load taxonomy: %v", err)
	}

	// Check sectors
	if len(tax.Sectors) != 2 {
		t.Errorf("Expected 2 sectors, got %d", len(tax.Sectors))
	}
	if len(tax.Sectors["ai"]) != 2 {
		t.Error("AI sector should have 2 keywords")
	}

	// Check events
	if len(tax.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(tax.Events))
	}

	// Check regions
	if len(tax.Regions) != 1 {
		t.Errorf("Expected 1 region, got %d", len(tax.Regions))
	}

	// Check entities
	if len(tax.Entities) != 1 {
		t.Error("Expected 1 entity type")
	}
	if len(tax.Entities["tickers"]) != 1 {
		t.Error("Expected 1 ticker")
	}
}

func TestLoadDict(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dict.txt")

	content := `# Comment
machine learning|ml|ai
neural network|nn|ai
open source|oss|tech
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	dict, err := LoadDict(path)
	if err != nil {
		t.Fatalf("Failed to load dict: %v", err)
	}

	if len(dict.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(dict.Entries))
	}

	// Check first entry
	entry := dict.Entries[0]
	if entry.Canonical != "machine learning" {
		t.Errorf("Expected 'machine learning', got '%s'", entry.Canonical)
	}
	if len(entry.Variants) != 1 || entry.Variants[0] != "ml" {
		t.Error("Expected variant 'ml'")
	}
	if entry.Category != "ai" {
		t.Errorf("Expected category 'ai', got '%s'", entry.Category)
	}
}

func TestLoadDictWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dict.txt")

	content := `# This is a comment
machine learning|ml|ai
# Another comment

neural network|nn|ai
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	dict, err := LoadDict(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should skip comments and empty lines
	if len(dict.Entries) != 2 {
		t.Errorf("Expected 2 entries (comments skipped), got %d", len(dict.Entries))
	}
}

func TestLoadDictInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dict.txt")

	// Only one part (no pipes) - invalid
	content := `machinelearning`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	dict, err := LoadDict(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should skip invalid lines
	if len(dict.Entries) != 0 {
		t.Error("Invalid lines (no pipes) should be skipped")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := LoadStoplist("/nonexistent/path.yaml")
	if err == nil {
		t.Error("Should error on non-existent file")
	}

	_, err = LoadTaxonomy("/nonexistent/path.yaml")
	if err == nil {
		t.Error("Should error on non-existent file")
	}

	_, err = LoadDict("/nonexistent/path.txt")
	if err == nil {
		t.Error("Should error on non-existent file")
	}
}

func TestLoadEmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty stoplist
	slPath := filepath.Join(tmpDir, "empty_stoplist.yaml")
	os.WriteFile(slPath, []byte("terms: []"), 0644)
	sl, err := LoadStoplist(slPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(sl.Terms) != 0 {
		t.Error("Empty stoplist should have no terms")
	}

	// Empty dict
	dictPath := filepath.Join(tmpDir, "empty_dict.txt")
	os.WriteFile(dictPath, []byte(""), 0644)
	dict, err := LoadDict(dictPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(dict.Entries) != 0 {
		t.Error("Empty dict should have no entries")
	}
}
