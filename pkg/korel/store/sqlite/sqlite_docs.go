package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// UpsertDoc inserts or updates a document
func (s *sqliteStore) UpsertDoc(ctx context.Context, d store.Doc) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const stmt = `
INSERT INTO docs (url, title, body_snippet, outlet, published_at, links_out)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(url) DO UPDATE SET
	title=excluded.title,
	body_snippet=excluded.body_snippet,
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
		d.BodySnippet,
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

	s.statsValid = false // invalidate cached corpus stats
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

// DocCount returns the total number of documents (cached after first call).
func (s *sqliteStore) DocCount(ctx context.Context) (int64, error) {
	if s.statsValid {
		return s.cachedDocCount, nil
	}
	if err := s.warmStats(ctx); err != nil {
		return 0, err
	}
	return s.cachedDocCount, nil
}

// AvgDocLen returns the average number of unique tokens per document (cached after first call).
func (s *sqliteStore) AvgDocLen(ctx context.Context) (float64, error) {
	if s.statsValid {
		return s.cachedAvgLen, nil
	}
	if err := s.warmStats(ctx); err != nil {
		return 0, err
	}
	return s.cachedAvgLen, nil
}

func (s *sqliteStore) warmStats(ctx context.Context) error {
	var n int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM docs").Scan(&n); err != nil {
		return err
	}
	var avg sql.NullFloat64
	if err := s.db.QueryRowContext(ctx,
		"SELECT AVG(cnt) FROM (SELECT COUNT(*) AS cnt FROM doc_tokens GROUP BY doc_id)").Scan(&avg); err != nil {
		return err
	}
	s.cachedDocCount = n
	if avg.Valid {
		s.cachedAvgLen = avg.Float64
	}
	s.statsValid = true
	return nil
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
SELECT d.id, COUNT(DISTINCT dt.token) AS match_count
FROM docs d
JOIN doc_tokens dt ON d.id = dt.doc_id
WHERE dt.token IN (%s)
GROUP BY d.id
ORDER BY match_count DESC, d.published_at DESC
LIMIT ?;
`, placeholders)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		var matchCount int
		if err := rows.Scan(&id, &matchCount); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.loadDocsBatch(ctx, ids)
}

// GetDocsByTokensInRange retrieves documents containing any of the given tokens
// within the specified time range. Zero-value since/until means no bound.
func (s *sqliteStore) GetDocsByTokensInRange(ctx context.Context, tokens []string, since, until time.Time, limit int) ([]store.Doc, error) {
	unique := uniqueStrings(tokens)
	if len(unique) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	placeholders := strings.Repeat("?,", len(unique))
	placeholders = strings.TrimSuffix(placeholders, ",")

	args := make([]interface{}, 0, len(unique)+3)
	for _, tok := range unique {
		args = append(args, tok)
	}

	var whereClauses []string
	whereClauses = append(whereClauses, fmt.Sprintf("dt.token IN (%s)", placeholders))

	if !since.IsZero() {
		whereClauses = append(whereClauses, "d.published_at >= ?")
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	if !until.IsZero() {
		whereClauses = append(whereClauses, "d.published_at <= ?")
		args = append(args, until.UTC().Format(time.RFC3339))
	}

	args = append(args, limit)
	query := fmt.Sprintf(`
SELECT DISTINCT d.id
FROM docs d
JOIN doc_tokens dt ON d.id = dt.doc_id
WHERE %s
ORDER BY d.published_at DESC
LIMIT ?;
`, strings.Join(whereClauses, " AND "))

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

// GetDocsByEntity retrieves documents mentioning a specific entity.
// If entityType is empty, matches any entity type with the given value.
func (s *sqliteStore) GetDocsByEntity(ctx context.Context, entityType, entityValue string, limit int) ([]store.Doc, error) {
	if entityValue == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	var query string
	var args []interface{}
	if entityType != "" {
		query = `
SELECT DISTINCT d.id
FROM docs d
JOIN doc_entities de ON d.id = de.doc_id
WHERE de.type = ? AND de.value = ?
ORDER BY d.published_at DESC
LIMIT ?;`
		args = []interface{}{entityType, entityValue, limit}
	} else {
		query = `
SELECT DISTINCT d.id
FROM docs d
JOIN doc_entities de ON d.id = de.doc_id
WHERE de.value = ?
ORDER BY d.published_at DESC
LIMIT ?;`
		args = []interface{}{entityValue, limit}
	}

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

// loadDocsBatch loads multiple documents efficiently using batch queries instead of N+1.
func (s *sqliteStore) loadDocsBatch(ctx context.Context, ids []int64) ([]store.Doc, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build placeholders
	ph := strings.Repeat("?,", len(ids))
	ph = strings.TrimSuffix(ph, ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	// 1. Load all doc metadata in one query
	docMap := make(map[int64]*store.Doc, len(ids))
	orderMap := make(map[int64]int, len(ids))
	for i, id := range ids {
		orderMap[id] = i
	}

	q := fmt.Sprintf(`SELECT id, url, title, COALESCE(body_snippet, ''), outlet, published_at, links_out FROM docs WHERE id IN (%s)`, ph)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var doc store.Doc
		var published string
		if err := rows.Scan(&doc.ID, &doc.URL, &doc.Title, &doc.BodySnippet, &doc.Outlet, &published, &doc.LinksOut); err != nil {
			return nil, err
		}
		if published != "" {
			if parsed, perr := time.Parse(time.RFC3339, published); perr == nil {
				doc.PublishedAt = parsed
			}
		}
		docMap[doc.ID] = &doc
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. Load all tokens in one query
	q = fmt.Sprintf(`SELECT doc_id, token FROM doc_tokens WHERE doc_id IN (%s)`, ph)
	trows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var docID int64
		var token string
		if err := trows.Scan(&docID, &token); err != nil {
			return nil, err
		}
		if doc, ok := docMap[docID]; ok {
			doc.Tokens = append(doc.Tokens, token)
		}
	}
	if err := trows.Err(); err != nil {
		return nil, err
	}

	// 3. Load all categories in one query
	q = fmt.Sprintf(`SELECT doc_id, category FROM doc_cats WHERE doc_id IN (%s)`, ph)
	crows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var docID int64
		var cat string
		if err := crows.Scan(&docID, &cat); err != nil {
			return nil, err
		}
		if doc, ok := docMap[docID]; ok {
			doc.Cats = append(doc.Cats, cat)
		}
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}

	// 4. Load all entities in one query
	q = fmt.Sprintf(`SELECT doc_id, type, value FROM doc_entities WHERE doc_id IN (%s)`, ph)
	erows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var docID int64
		var ent store.Entity
		if err := erows.Scan(&docID, &ent.Type, &ent.Value); err != nil {
			return nil, err
		}
		if doc, ok := docMap[docID]; ok {
			doc.Ents = append(doc.Ents, ent)
		}
	}
	if err := erows.Err(); err != nil {
		return nil, err
	}

	// Preserve original order
	results := make([]store.Doc, 0, len(ids))
	for _, id := range ids {
		if doc, ok := docMap[id]; ok {
			results = append(results, *doc)
		}
	}
	return results, nil
}

func (s *sqliteStore) loadDoc(ctx context.Context, id int64) (store.Doc, error) {
	var (
		doc       store.Doc
		published string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT id, url, title, COALESCE(body_snippet, ''), outlet, published_at, links_out
FROM docs
WHERE id = ?;
`, id).Scan(&doc.ID, &doc.URL, &doc.Title, &doc.BodySnippet, &doc.Outlet, &published, &doc.LinksOut)
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
