package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store"
)

// sqliteStore implements the Store interface using SQLite
type sqliteStore struct {
	db     *sql.DB
	pmiCfg pmi.Config
	calc   *pmi.Calculator
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
`

	_, err := db.ExecContext(ctx, schema)
	return err
}

// UpsertDoc inserts or updates a document
func (s *sqliteStore) UpsertDoc(ctx context.Context, d store.Doc) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const stmt = `
INSERT INTO docs (url, title, outlet, published_at, links_out)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(url) DO UPDATE SET
	title=excluded.title,
	outlet=excluded.outlet,
	published_at=excluded.published_at,
	links_out=excluded.links_out
RETURNING id;
`

	var docID int64
	err = tx.QueryRowContext(
		ctx,
		stmt,
		d.URL,
		d.Title,
		d.Outlet,
		d.PublishedAt.UTC().Format(time.RFC3339),
		d.LinksOut,
	).Scan(&docID)
	if err != nil {
		return err
	}

	if err := replaceDocTokens(ctx, tx, docID, uniqueStrings(d.Tokens)); err != nil {
		return err
	}
	if err := replaceDocCategories(ctx, tx, docID, uniqueStrings(d.Cats)); err != nil {
		return err
	}
	if err := replaceDocEntities(ctx, tx, docID, uniqueEntities(d.Ents)); err != nil {
		return err
	}

	return tx.Commit()
}

func replaceDocTokens(ctx context.Context, tx *sql.Tx, docID int64, tokens []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_tokens WHERE doc_id=?`, docID); err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO doc_tokens (doc_id, token) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, docID, tok); err != nil {
			return err
		}
	}
	return nil
}

func replaceDocCategories(ctx context.Context, tx *sql.Tx, docID int64, cats []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_cats WHERE doc_id=?`, docID); err != nil {
		return err
	}
	if len(cats) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO doc_cats (doc_id, category) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, cat := range cats {
		if cat == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, docID, cat); err != nil {
			return err
		}
	}
	return nil
}

func replaceDocEntities(ctx context.Context, tx *sql.Tx, docID int64, ents []store.Entity) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_entities WHERE doc_id=?`, docID); err != nil {
		return err
	}
	if len(ents) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO doc_entities (doc_id, type, value) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ent := range ents {
		if ent.Type == "" || ent.Value == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, docID, ent.Type, ent.Value); err != nil {
			return err
		}
	}
	return nil
}

// GetDoc retrieves a document by ID
func (s *sqliteStore) GetDoc(ctx context.Context, id int64) (store.Doc, error) {
	return s.loadDoc(ctx, id)
}

// GetDocByURL retrieves a document by URL
func (s *sqliteStore) GetDocByURL(ctx context.Context, url string) (store.Doc, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM docs WHERE url = ?`, url).Scan(&id)
	if err == sql.ErrNoRows {
		return store.Doc{}, false, nil
	}
	if err != nil {
		return store.Doc{}, false, err
	}

	doc, err := s.loadDoc(ctx, id)
	if err != nil {
		return store.Doc{}, false, err
	}
	return doc, true, nil
}

// GetDocsByTokens retrieves documents containing any of the given tokens
func (s *sqliteStore) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]store.Doc, error) {
	unique := uniqueStrings(tokens)
	if len(unique) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	placeholders := strings.Repeat("?,", len(unique))
	placeholders = strings.TrimSuffix(placeholders, ",")

	args := make([]interface{}, 0, len(unique)+1)
	for _, tok := range unique {
		args = append(args, tok)
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
SELECT DISTINCT d.id
FROM docs d
JOIN doc_tokens dt ON d.id = dt.doc_id
WHERE dt.token IN (%s)
ORDER BY d.published_at DESC
LIMIT ?;
`, placeholders)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.Doc
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		doc, err := s.loadDoc(ctx, id)
		if err != nil {
			return nil, err
		}
		results = append(results, doc)
	}
	return results, rows.Err()
}

// UpsertTokenDF updates the document frequency for a token
func (s *sqliteStore) UpsertTokenDF(ctx context.Context, token string, df int64) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO token_df (token, df) VALUES (?, ?)
ON CONFLICT(token) DO UPDATE SET df=excluded.df;
`, token, df)
	return err
}

// GetTokenDF retrieves the document frequency for a token
func (s *sqliteStore) GetTokenDF(ctx context.Context, token string) (int64, error) {
	var df int64
	err := s.db.QueryRowContext(ctx, `SELECT df FROM token_df WHERE token=?`, token).Scan(&df)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return df, err
}

// IncPair increments the co-occurrence count for a token pair
func (s *sqliteStore) IncPair(ctx context.Context, t1, t2 string) error {
	if t1 == t2 {
		return nil
	}
	if t1 > t2 {
		t1, t2 = t2, t1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO token_pairs (t1, t2, count) VALUES (?, ?, 1)
ON CONFLICT(t1, t2) DO UPDATE SET count=count+1;
`, t1, t2)
	return err
}

// DecPair decrements the co-occurrence count for a token pair
func (s *sqliteStore) DecPair(ctx context.Context, t1, t2 string) error {
	if t1 == t2 {
		return nil
	}
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	if _, err := s.db.ExecContext(ctx, `
UPDATE token_pairs
SET count = CASE WHEN count > 0 THEN count - 1 ELSE 0 END
WHERE t1 = ? AND t2 = ?;
`, t1, t2); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
DELETE FROM token_pairs
WHERE t1 = ? AND t2 = ? AND count <= 0;
`, t1, t2)
	return err
}

// GetPMI retrieves the PMI score for a token pair
func (s *sqliteStore) GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error) {
	if t1 == t2 {
		return 0, false, nil
	}
	a, b := t1, t2
	if a > b {
		a, b = b, a
	}

	var co int64
	err := s.db.QueryRowContext(ctx, `SELECT count FROM token_pairs WHERE t1=? AND t2=?`, a, b).Scan(&co)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	dfA, err := s.GetTokenDF(ctx, t1)
	if err != nil {
		return 0, false, err
	}
	dfB, err := s.GetTokenDF(ctx, t2)
	if err != nil {
		return 0, false, err
	}
	total, err := s.totalDocs(ctx)
	if err != nil {
		return 0, false, err
	}
	if total == 0 {
		return 0, false, nil
	}

	score := s.calc.Score(co, dfA, dfB, total, s.pmiCfg.UseNPMI)
	return score, true, nil
}

// TopNeighbors returns the top K neighbors ranked by PMI score for a token.
func (s *sqliteStore) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {
	if k <= 0 {
		k = 10
	}

	total, err := s.totalDocs(ctx)
	if err != nil || total == 0 {
		return nil, err
	}

	dfToken, err := s.GetTokenDF(ctx, token)
	if err != nil || dfToken == 0 {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT
	CASE WHEN t1 = ? THEN t2 ELSE t1 END AS neighbor,
	count
FROM token_pairs
WHERE t1 = ? OR t2 = ?;
`, token, token, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type raw struct {
		token string
		count int64
	}
	var pairs []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.token, &r.count); err != nil {
			return nil, err
		}
		pairs = append(pairs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Compute PMI/NPMI for each neighbor and collect into result.
	var neighbors []store.Neighbor
	for _, r := range pairs {
		dfOther, err := s.GetTokenDF(ctx, r.token)
		if err != nil || dfOther < s.pmiCfg.MinDF {
			continue // skip rare words — PMI over-rewards low-frequency terms
		}
		score := s.calc.Score(r.count, dfToken, dfOther, total, s.pmiCfg.UseNPMI)
		neighbors = append(neighbors, store.Neighbor{Token: r.token, PMI: score})
	}

	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].PMI > neighbors[j].PMI
	})
	if len(neighbors) > k {
		neighbors = neighbors[:k]
	}
	return neighbors, nil
}

// UpsertCard inserts or updates a card
func (s *sqliteStore) UpsertCard(ctx context.Context, c store.Card) error {
	bulletsJSON, err := json.Marshal(c.Bullets)
	if err != nil {
		return err
	}
	sourcesJSON, err := json.Marshal(c.Sources)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO cards (id, title, bullets, sources, score_json, period)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	title=excluded.title,
	bullets=excluded.bullets,
	sources=excluded.sources,
	score_json=excluded.score_json,
	period=excluded.period;
`, c.ID, c.Title, string(bulletsJSON), string(sourcesJSON), c.ScoreJSON, c.Period)
	return err
}

// GetCardsByPeriod retrieves cards for a given time period
func (s *sqliteStore) GetCardsByPeriod(ctx context.Context, period string, k int) ([]store.Card, error) {
	if k <= 0 {
		k = 10
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, bullets, sources, score_json, period
FROM cards
WHERE period = ?
ORDER BY id DESC
LIMIT ?;
`, period, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []store.Card
	for rows.Next() {
		var c store.Card
		var bulletsJSON, sourcesJSON string
		if err := rows.Scan(&c.ID, &c.Title, &bulletsJSON, &sourcesJSON, &c.ScoreJSON, &c.Period); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(bulletsJSON), &c.Bullets); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(sourcesJSON), &c.Sources); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// Stoplist returns a view of the stopword list
func (s *sqliteStore) Stoplist() store.StoplistView {
	return nil
}

// Dict returns a view of the multi-token dictionary
func (s *sqliteStore) Dict() store.DictView {
	return nil
}

// Taxonomy returns a view of the taxonomy
func (s *sqliteStore) Taxonomy() store.TaxonomyView {
	return nil
}

func (s *sqliteStore) loadDoc(ctx context.Context, id int64) (store.Doc, error) {
	var (
		doc       store.Doc
		published string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT id, url, title, outlet, published_at, links_out
FROM docs
WHERE id = ?;
`, id).Scan(&doc.ID, &doc.URL, &doc.Title, &doc.Outlet, &published, &doc.LinksOut)
	if err != nil {
		return store.Doc{}, err
	}

	if published != "" {
		if parsed, perr := time.Parse(time.RFC3339, published); perr == nil {
			doc.PublishedAt = parsed
		}
	}

	doc.Tokens, err = s.loadStringColumn(ctx, `SELECT token FROM doc_tokens WHERE doc_id=?`, id)
	if err != nil {
		return store.Doc{}, err
	}
	doc.Cats, err = s.loadStringColumn(ctx, `SELECT category FROM doc_cats WHERE doc_id=?`, id)
	if err != nil {
		return store.Doc{}, err
	}
	doc.Ents, err = s.loadEntities(ctx, id)
	if err != nil {
		return store.Doc{}, err
	}

	return doc, nil
}

func (s *sqliteStore) loadStringColumn(ctx context.Context, query string, args ...interface{}) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, rows.Err()
}

func (s *sqliteStore) loadEntities(ctx context.Context, docID int64) ([]store.Entity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type, value FROM doc_entities WHERE doc_id=?`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ents []store.Entity
	for rows.Next() {
		var ent store.Entity
		if err := rows.Scan(&ent.Type, &ent.Value); err != nil {
			return nil, err
		}
		ents = append(ents, ent)
	}
	return ents, rows.Err()
}

func (s *sqliteStore) totalDocs(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs`).Scan(&total)
	return total, err
}

func uniqueStrings(in []string) []string {
	set := make(map[string]struct{}, len(in))
	var out []string
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := set[v]; ok {
			continue
		}
		set[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func uniqueEntities(in []store.Entity) []store.Entity {
	type key struct {
		Type  string
		Value string
	}
	set := make(map[key]struct{}, len(in))
	var out []store.Entity
	for _, e := range in {
		if e.Type == "" || e.Value == "" {
			continue
		}
		k := key{Type: e.Type, Value: e.Value}
		if _, ok := set[k]; ok {
			continue
		}
		set[k] = struct{}{}
		out = append(out, e)
	}
	return out
}
