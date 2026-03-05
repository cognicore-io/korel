package sqlite

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"

	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store"
)

// sqliteStore implements the Store interface using SQLite
type sqliteStore struct {
	db     *sql.DB
	pmiCfg pmi.Config
	calc   *pmi.Calculator

	// Cached corpus stats (computed once, cleared on UpsertDoc)
	cachedDocCount int64
	cachedAvgLen   float64
	statsValid     bool
}

// OpenSQLite opens a SQLite database with WAL mode enabled.
// An optional pmi.Config can be passed to control PMI computation;
// if omitted, pmi.DefaultConfig() is used.
func OpenSQLite(ctx context.Context, path string, cfg ...pmi.Config) (store.Store, error) {
	c := pmi.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, err
	}

	// Initialize schema
	if err := initSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return &sqliteStore{
		db:     db,
		pmiCfg: c,
		calc:   pmi.NewCalculatorFromConfig(c),
	}, nil
}

// Close closes the database connection
func (s *sqliteStore) Close() error {
	return s.db.Close()
}

// initSchema creates tables if they don't exist
func initSchema(ctx context.Context, db *sql.DB) error {
	schema := `
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

CREATE TABLE IF NOT EXISTS doc_cats (
	doc_id INTEGER NOT NULL,
	category TEXT NOT NULL,
	UNIQUE(doc_id, category),
	FOREIGN KEY(doc_id) REFERENCES docs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS doc_entities (
	doc_id INTEGER NOT NULL,
	type TEXT NOT NULL,
	value TEXT NOT NULL,
	UNIQUE(doc_id, type, value),
	FOREIGN KEY(doc_id) REFERENCES docs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS token_df (
	token TEXT PRIMARY KEY,
	df INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS token_pairs (
	t1 TEXT NOT NULL,
	t2 TEXT NOT NULL,
	count INTEGER NOT NULL,
	PRIMARY KEY(t1, t2)
);

CREATE TABLE IF NOT EXISTS cards (
	id TEXT PRIMARY KEY,
	title TEXT,
	bullets TEXT,
	sources TEXT,
	score_json TEXT,
	period TEXT
);

CREATE TABLE IF NOT EXISTS stoplist (
	token TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS dict_entries (
	phrase TEXT PRIMARY KEY,
	canonical TEXT NOT NULL,
	category TEXT
);

CREATE TABLE IF NOT EXISTS taxonomy_sectors (
	name TEXT NOT NULL,
	keyword TEXT NOT NULL,
	PRIMARY KEY(name, keyword)
);

CREATE TABLE IF NOT EXISTS taxonomy_events (
	name TEXT NOT NULL,
	keyword TEXT NOT NULL,
	PRIMARY KEY(name, keyword)
);

CREATE TABLE IF NOT EXISTS taxonomy_regions (
	name TEXT NOT NULL,
	keyword TEXT NOT NULL,
	PRIMARY KEY(name, keyword)
);

CREATE TABLE IF NOT EXISTS taxonomy_entities (
	type TEXT NOT NULL,
	name TEXT NOT NULL,
	keyword TEXT NOT NULL,
	PRIMARY KEY(type, name, keyword)
);

CREATE TABLE IF NOT EXISTS edges (
	subject TEXT NOT NULL,
	relation TEXT NOT NULL,
	object TEXT NOT NULL,
	weight REAL NOT NULL DEFAULT 1.0,
	source TEXT NOT NULL,
	PRIMARY KEY(subject, relation, object)
);
CREATE INDEX IF NOT EXISTS idx_edges_subject ON edges(subject);
CREATE INDEX IF NOT EXISTS idx_edges_object ON edges(object);
`

	_, err := db.ExecContext(ctx, schema)
	return err
}

// migrateSchema adds columns introduced after the initial schema.
func migrateSchema(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('docs') WHERE name='body_snippet'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.ExecContext(ctx, `ALTER TABLE docs ADD COLUMN body_snippet TEXT DEFAULT ''`)
		if err != nil {
			return err
		}
	}

	// Migrate: add edges table if missing.
	var edgesExists int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='edges'`).Scan(&edgesExists)
	if err != nil {
		return err
	}
	if edgesExists == 0 {
		_, err = db.ExecContext(ctx, `
CREATE TABLE edges (
	subject TEXT NOT NULL,
	relation TEXT NOT NULL,
	object TEXT NOT NULL,
	weight REAL NOT NULL DEFAULT 1.0,
	source TEXT NOT NULL,
	PRIMARY KEY(subject, relation, object)
);
CREATE INDEX idx_edges_subject ON edges(subject);
CREATE INDEX idx_edges_object ON edges(object);
`)
		if err != nil {
			return err
		}
	}

	// Migrate: add feedback table if missing.
	var feedbackExists int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='feedback'`).Scan(&feedbackExists)
	if err != nil {
		return err
	}
	if feedbackExists == 0 {
		_, err = db.ExecContext(ctx, `
CREATE TABLE feedback (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	query_hash TEXT NOT NULL,
	card_id TEXT NOT NULL,
	action TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX idx_feedback_action ON feedback(action);
`)
		if err != nil {
			return err
		}
	}

	return nil
}
