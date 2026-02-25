package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// TestSchemaCreationIdempotent tests that running initSchema multiple times is safe
func TestSchemaCreationIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}
	defer db.Close()

	// Run schema initialization multiple times
	for i := 0; i < 3; i++ {
		if err := initSchema(ctx, db); err != nil {
			t.Fatalf("initSchema iteration %d: %v", i, err)
		}
	}

	// Verify schema is correct
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count)
	if err != nil {
		t.Fatalf("Count tables: %v", err)
	}

	expected := 13 // docs, doc_tokens, doc_cats, doc_entities, token_df, token_pairs, cards, stoplist, dict_entries, taxonomy_sectors, taxonomy_events, taxonomy_regions, taxonomy_entities
	if count != expected {
		t.Errorf("Expected %d tables, got %d", expected, count)
	}
}

// TestMigrationPreservesData tests that schema changes preserve existing data
func TestMigrationPreservesData(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create initial database and insert data
	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	// Insert test document
	doc := store.Doc{
		URL:         "https://example.com/test",
		Title:       "Test Document",
		Outlet:      "Test Outlet",
		PublishedAt: time.Now(),
		Tokens:      []string{"machine", "learning"},
		Cats:        []string{"ai"},
		LinksOut:    5,
	}

	if err := st.UpsertDoc(ctx, doc); err != nil {
		t.Fatalf("UpsertDoc: %v", err)
	}

	// Insert token stats
	st.UpsertTokenDF(ctx, "machine", 10)
	st.UpsertTokenDF(ctx, "learning", 15)
	st.IncPair(ctx, "machine", "learning")

	st.Close()

	// Reopen database (simulates upgrade)
	st2, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("Reopen database: %v", err)
	}
	defer st2.Close()

	// Verify document preserved
	retrieved, found, err := st2.GetDocByURL(ctx, doc.URL)
	if err != nil {
		t.Fatalf("GetDocByURL: %v", err)
	}

	if !found {
		t.Fatal("Document should be preserved after migration")
	}

	if retrieved.Title != doc.Title {
		t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, doc.Title)
	}

	// Verify token stats preserved
	df, err := st2.GetTokenDF(ctx, "machine")
	if err != nil {
		t.Fatalf("GetTokenDF: %v", err)
	}

	if df != 10 {
		t.Errorf("Expected DF=10, got %d", df)
	}

	// Verify pair preserved
	_, ok, err := st2.GetPMI(ctx, "machine", "learning")
	if err != nil {
		t.Fatalf("GetPMI: %v", err)
	}

	if !ok {
		t.Error("Pair should be preserved after migration")
	}
}

// TestBackwardCompatibility tests that newer code can read old database format
func TestBackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "old.db")

	// Simulate old database format (same as current for now)
	// In real migration scenarios, this would create an older schema version
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}

	// Create "old" schema (current schema minus potential future additions)
	oldSchema := `
CREATE TABLE IF NOT EXISTS docs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT UNIQUE NOT NULL,
	title TEXT,
	outlet TEXT,
	published_at TEXT,
	links_out INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS doc_tokens (
	doc_id INTEGER NOT NULL,
	token TEXT NOT NULL,
	UNIQUE(doc_id, token),
	FOREIGN KEY(doc_id) REFERENCES docs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS token_df (
	token TEXT PRIMARY KEY,
	df INTEGER NOT NULL
);
`

	if _, err := db.ExecContext(ctx, oldSchema); err != nil {
		t.Fatalf("Create old schema: %v", err)
	}

	// Insert data using old schema
	result, err := db.ExecContext(ctx, "INSERT INTO docs (url, title, outlet, published_at, links_out) VALUES (?, ?, ?, ?, ?)",
		"https://example.com/old", "Old Doc", "Old Outlet", time.Now().Format(time.RFC3339), 3)
	if err != nil {
		t.Fatalf("Insert old doc: %v", err)
	}

	docID, _ := result.LastInsertId()
	db.ExecContext(ctx, "INSERT INTO doc_tokens (doc_id, token) VALUES (?, ?)", docID, "old")
	db.ExecContext(ctx, "INSERT INTO token_df (token, df) VALUES (?, ?)", "old", 5)

	db.Close()

	// Now open with new code (should handle migration)
	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite with old DB: %v", err)
	}
	defer st.Close()

	// Verify old data is readable
	doc, found, err := st.GetDocByURL(ctx, "https://example.com/old")
	if err != nil {
		t.Fatalf("GetDocByURL: %v", err)
	}

	if !found {
		t.Fatal("Old document should be readable")
	}

	if doc.Title != "Old Doc" {
		t.Errorf("Title mismatch: got %q, want %q", doc.Title, "Old Doc")
	}

	// Verify token DF
	df, err := st.GetTokenDF(ctx, "old")
	if err != nil {
		t.Fatalf("GetTokenDF: %v", err)
	}

	if df != 5 {
		t.Errorf("Expected DF=5, got %d", df)
	}

	// Verify new schema elements exist (doc_cats, doc_entities, token_pairs, cards)
	sqliteStore := st.(*sqliteStore)
	tables := []string{"doc_cats", "doc_entities", "token_pairs", "cards"}
	for _, table := range tables {
		var exists int
		err := sqliteStore.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("Check table %s: %v", table, err)
		}

		if exists == 0 {
			t.Errorf("Table %s should exist after opening old DB", table)
		}
	}
}

// TestSchemaVersion tests that we can track schema version for future migrations
func TestSchemaVersion(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()

	// In the future, we might add a version table like:
	// CREATE TABLE schema_version (version INTEGER PRIMARY KEY)
	// For now, just verify the schema is stable

	sqliteStore := st.(*sqliteStore)

	// Verify all expected tables exist
	expectedTables := []string{
		"docs",
		"doc_tokens",
		"doc_cats",
		"doc_entities",
		"token_df",
		"token_pairs",
		"cards",
	}

	for _, table := range expectedTables {
		var exists int
		err := sqliteStore.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("Check table %s: %v", table, err)
		}

		if exists == 0 {
			t.Errorf("Table %s should exist", table)
		}
	}
}

// TestConcurrentOpen tests that after initial schema creation, multiple connections work
func TestConcurrentOpen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// First, create the schema (single connection)
	initial, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("Initial OpenSQLite: %v", err)
	}
	initial.Close()

	// Give SQLite time to release locks
	time.Sleep(50 * time.Millisecond)

	// Now open multiple connections to existing database
	errChan := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			st, err := OpenSQLite(ctx, dbPath)
			if err != nil {
				errChan <- err
				return
			}
			// Try a simple operation
			_, _ = st.GetTokenDF(ctx, "test")
			st.Close()
			errChan <- nil
		}()
	}

	// Collect errors
	for i := 0; i < 5; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent open failed: %v", err)
		}
	}

	// Verify schema is still correct
	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("Final open: %v", err)
	}
	defer st.Close()

	sqliteStore := st.(*sqliteStore)
	var count int
	err = sqliteStore.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count)
	if err != nil {
		t.Fatalf("Count tables: %v", err)
	}

	if count != 13 {
		t.Errorf("Expected 13 tables, got %d", count)
	}
}
