package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildEngine tests that buildEngine correctly loads configuration
func TestBuildEngine(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use existing test fixtures
	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"
	rulesPath := ""

	engine, cleanup, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, rulesPath)
	if err != nil {
		t.Fatalf("buildEngine failed: %v", err)
	}
	defer cleanup()

	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}
}

// TestBuildEngineNonExistentStoplist tests that buildEngine fails with missing stoplist
func TestBuildEngineNonExistentStoplist(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := filepath.Join(tmpDir, "nonexistent.yaml")
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	_, _, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err == nil {
		t.Error("buildEngine should fail with non-existent stoplist")
	}
}

// TestBuildEngineNonExistentDict tests that buildEngine fails with missing dict
func TestBuildEngineNonExistentDict(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := filepath.Join(tmpDir, "nonexistent.dict")
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	_, _, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err == nil {
		t.Error("buildEngine should fail with non-existent dict")
	}
}

// TestBuildEngineNonExistentTaxonomy tests that buildEngine fails with missing taxonomy
func TestBuildEngineNonExistentTaxonomy(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := filepath.Join(tmpDir, "nonexistent.yaml")

	_, _, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err == nil {
		t.Error("buildEngine should fail with non-existent taxonomy")
	}
}

// TestBuildEngineWithRules tests that buildEngine loads rules when provided
func TestBuildEngineWithRules(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test rules file
	rulesPath := filepath.Join(tmpDir, "test.rules")
	rulesContent := `is_a(bert, transformer).
is_a(transformer, neural-network).
related_to(bert, nlp).
`
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("Failed to create rules file: %v", err)
	}

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	engine, cleanup, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, rulesPath)
	if err != nil {
		t.Fatalf("buildEngine with rules failed: %v", err)
	}
	defer cleanup()

	if engine == nil {
		t.Fatal("Expected non-nil engine with rules")
	}
}

// TestBuildEngineInvalidDBPath tests that buildEngine fails gracefully with invalid DB path
func TestBuildEngineInvalidDBPath(t *testing.T) {
	ctx := context.Background()

	// Use a path in a non-existent directory
	dbPath := "/nonexistent/directory/test.db"
	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	_, _, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err == nil {
		t.Error("buildEngine should fail with invalid DB path")
	}
}

// TestBuildEngineMalformedRules tests that buildEngine fails with malformed rules
func TestBuildEngineMalformedRules(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a malformed rules file
	rulesPath := filepath.Join(tmpDir, "bad.rules")
	badRules := `this is not valid prolog syntax!@#$`
	if err := os.WriteFile(rulesPath, []byte(badRules), 0644); err != nil {
		t.Fatalf("Failed to create rules file: %v", err)
	}

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	_, _, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, rulesPath)
	if err == nil {
		t.Error("buildEngine should fail with malformed rules")
	}
}

// TestExecuteQueryEmptyDB tests query execution on empty database
func TestExecuteQueryEmptyDB(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	engine, cleanup, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err != nil {
		t.Fatalf("buildEngine failed: %v", err)
	}
	defer cleanup()

	// Execute query on empty database - should return no results without error
	err = executeQuery(ctx, engine, "machine learning", 3)
	if err != nil {
		t.Errorf("executeQuery should not fail on empty DB: %v", err)
	}
}

// TestExecuteQueryDifferentTopK tests query execution with different topK values
func TestExecuteQueryDifferentTopK(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	engine, cleanup, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err != nil {
		t.Fatalf("buildEngine failed: %v", err)
	}
	defer cleanup()

	testCases := []int{1, 3, 5, 10}
	for _, topK := range testCases {
		err = executeQuery(ctx, engine, "test query", topK)
		if err != nil {
			t.Errorf("executeQuery with topK=%d failed: %v", topK, err)
		}
	}
}

// TestExecuteQueryEmptyString tests query execution with empty string
func TestExecuteQueryEmptyString(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	stoplistPath := "../../testdata/hn/stoplist.yaml"
	dictPath := "../../testdata/hn/tokens.dict"
	taxonomyPath := "../../testdata/hn/taxonomies.yaml"

	engine, cleanup, err := buildEngine(ctx, dbPath, stoplistPath, dictPath, taxonomyPath, "")
	if err != nil {
		t.Fatalf("buildEngine failed: %v", err)
	}
	defer cleanup()

	// Empty query should not crash
	err = executeQuery(ctx, engine, "", 3)
	if err != nil {
		t.Errorf("executeQuery with empty string should not fail: %v", err)
	}
}
