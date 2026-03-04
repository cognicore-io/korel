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

// DocCount returns the total number of documents.
func (s *sqliteStore) DocCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM docs").Scan(&n)
	return n, err
}

// AvgDocLen returns the average number of unique tokens per document.
func (s *sqliteStore) AvgDocLen(ctx context.Context) (float64, error) {
	var avg sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		"SELECT AVG(cnt) FROM (SELECT COUNT(*) AS cnt FROM doc_tokens GROUP BY doc_id)").Scan(&avg)
	if err != nil {
		return 0, err
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
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

	var results []store.Doc
	for rows.Next() {
		var id int64
		var matchCount int
		if err := rows.Scan(&id, &matchCount); err != nil {
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
