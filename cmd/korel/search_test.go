package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cognicore/korel/internal/bootstrap"
)

func testBootstrap(t *testing.T, opts bootstrap.Options) (*bootstrap.Result, error) {
	t.Helper()
	ctx := context.Background()
	return bootstrap.Run(ctx, opts)
}

func baseOpts(dbPath string) bootstrap.Options {
	return bootstrap.Options{
		DBPath:          dbPath,
		StoplistPath:    "../../testdata/hn/stoplist.yaml",
		DictPath:        "../../testdata/hn/tokens.dict",
		TaxonomyPath:    "../../testdata/hn/taxonomies.yaml",
		SimpleInference: true,
	}
}

// TestBootstrapEngine tests that bootstrap correctly loads configuration
func TestBootstrapEngine(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	res, err := testBootstrap(t, baseOpts(dbPath))
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	defer res.Close()

	if res.Engine == nil {
		t.Fatal("Expected non-nil engine")
	}
}

// TestBootstrapNonExistentStoplist tests that bootstrap fails with missing stoplist
func TestBootstrapNonExistentStoplist(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	opts.StoplistPath = filepath.Join(t.TempDir(), "nonexistent.yaml")

	_, err := testBootstrap(t, opts)
	if err == nil {
		t.Error("bootstrap should fail with non-existent stoplist")
	}
}

// TestBootstrapNonExistentDict tests that bootstrap fails with missing dict
func TestBootstrapNonExistentDict(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	opts.DictPath = filepath.Join(t.TempDir(), "nonexistent.dict")

	_, err := testBootstrap(t, opts)
	if err == nil {
		t.Error("bootstrap should fail with non-existent dict")
	}
}

// TestBootstrapNonExistentTaxonomy tests that bootstrap fails with missing taxonomy
func TestBootstrapNonExistentTaxonomy(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	opts.TaxonomyPath = filepath.Join(t.TempDir(), "nonexistent.yaml")

	_, err := testBootstrap(t, opts)
	if err == nil {
		t.Error("bootstrap should fail with non-existent taxonomy")
	}
}

// TestBootstrapWithRules tests that bootstrap loads rules when provided
func TestBootstrapWithRules(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "test.rules")
	rulesContent := `is_a(bert, transformer).
is_a(transformer, neural-network).
related_to(bert, nlp).
`
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("Failed to create rules file: %v", err)
	}

	opts := baseOpts(filepath.Join(tmpDir, "test.db"))
	opts.RulesPath = rulesPath

	res, err := testBootstrap(t, opts)
	if err != nil {
		t.Fatalf("bootstrap with rules failed: %v", err)
	}
	defer res.Close()

	if res.Engine == nil {
		t.Fatal("Expected non-nil engine with rules")
	}
}

// TestBootstrapInvalidDBPath tests that bootstrap fails gracefully with invalid DB path
func TestBootstrapInvalidDBPath(t *testing.T) {
	opts := baseOpts("/nonexistent/directory/test.db")

	_, err := testBootstrap(t, opts)
	if err == nil {
		t.Error("bootstrap should fail with invalid DB path")
	}
}

// TestBootstrapMalformedRules tests that bootstrap fails with malformed rules
func TestBootstrapMalformedRules(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "bad.rules")
	if err := os.WriteFile(rulesPath, []byte(`this is not valid prolog syntax!@#$`), 0644); err != nil {
		t.Fatalf("Failed to create rules file: %v", err)
	}

	opts := baseOpts(filepath.Join(tmpDir, "test.db"))
	opts.RulesPath = rulesPath

	_, err := testBootstrap(t, opts)
	if err == nil {
		t.Error("bootstrap should fail with malformed rules")
	}
}

// TestExecuteQueryEmptyDB tests query execution on empty database
func TestExecuteQueryEmptyDB(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	res, err := testBootstrap(t, opts)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	defer res.Close()

	ctx := context.Background()
	err = executeQuery(ctx, res.Engine, "machine learning", 3, searchOpts{})
	if err != nil {
		t.Errorf("executeQuery should not fail on empty DB: %v", err)
	}
}

// TestExecuteQueryDifferentTopK tests query execution with different topK values
func TestExecuteQueryDifferentTopK(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	res, err := testBootstrap(t, opts)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	defer res.Close()

	ctx := context.Background()
	for _, topK := range []int{1, 3, 5, 10} {
		err = executeQuery(ctx, res.Engine, "test query", topK, searchOpts{})
		if err != nil {
			t.Errorf("executeQuery with topK=%d failed: %v", topK, err)
		}
	}
}

// TestExecuteQueryEmptyString tests query execution with empty string
func TestExecuteQueryEmptyString(t *testing.T) {
	opts := baseOpts(filepath.Join(t.TempDir(), "test.db"))
	res, err := testBootstrap(t, opts)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	defer res.Close()

	ctx := context.Background()
	err = executeQuery(ctx, res.Engine, "", 3, searchOpts{})
	if err != nil {
		t.Errorf("executeQuery with empty string should not fail: %v", err)
	}
}
