package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cognicore/korel/pkg/korel/store"
)

// Stoplist returns a view of the stopword list.
// Returns nil if the stoplist table is empty.
func (s *sqliteStore) Stoplist() store.StoplistView {
	var count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM stoplist`).Scan(&count); err != nil || count == 0 {
		return nil
	}
	return &sqliteStoplistView{db: s.db}
}

// Dict returns a view of the multi-token dictionary.
// Returns nil if the dict_entries table is empty.
func (s *sqliteStore) Dict() store.DictView {
	var count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM dict_entries`).Scan(&count); err != nil || count == 0 {
		return nil
	}
	return &sqliteDictView{db: s.db}
}

// Taxonomy returns a view of the taxonomy.
// Returns nil if all taxonomy tables are empty.
func (s *sqliteStore) Taxonomy() store.TaxonomyView {
	var total int64
	for _, tbl := range []string{"taxonomy_sectors", "taxonomy_events", "taxonomy_regions", "taxonomy_entities"} {
		var c int64
		if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tbl)).Scan(&c); err == nil {
			total += c
		}
	}
	if total == 0 {
		return nil
	}
	return &sqliteTaxonomyView{db: s.db}
}

// UpsertStoplist replaces the stopword set in a single transaction.
func (s *sqliteStore) UpsertStoplist(ctx context.Context, tokens []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM stoplist`); err != nil {
		return err
	}

	if len(tokens) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO stoplist (token) VALUES (?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, tok := range tokens {
			if _, err := stmt.ExecContext(ctx, tok); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// UpsertDictEntry adds or replaces a dictionary entry.
func (s *sqliteStore) UpsertDictEntry(ctx context.Context, phrase, canonical, category string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO dict_entries (phrase, canonical, category) VALUES (?, ?, ?)
ON CONFLICT(phrase) DO UPDATE SET canonical=excluded.canonical, category=excluded.category;
`, phrase, canonical, category)
	return err
}

// --- SQLite StoplistView ---

type sqliteStoplistView struct{ db *sql.DB }

func (v *sqliteStoplistView) IsStop(token string) bool {
	var count int64
	if err := v.db.QueryRow(`SELECT COUNT(*) FROM stoplist WHERE token=?`, token).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (v *sqliteStoplistView) AllStops() []string {
	rows, err := v.db.Query(`SELECT token FROM stoplist ORDER BY token`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var stops []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return stops
		}
		stops = append(stops, tok)
	}
	return stops
}

// --- SQLite DictView ---

type sqliteDictView struct{ db *sql.DB }

func (v *sqliteDictView) Lookup(phrase string) (string, string, bool) {
	var canonical, category string
	err := v.db.QueryRow(`SELECT canonical, category FROM dict_entries WHERE phrase=?`, phrase).Scan(&canonical, &category)
	if err != nil {
		return "", "", false
	}
	return canonical, category, true
}

func (v *sqliteDictView) AllEntries() []store.DictEntryData {
	rows, err := v.db.Query(`SELECT phrase, canonical, category FROM dict_entries ORDER BY phrase`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var entries []store.DictEntryData
	for rows.Next() {
		var e store.DictEntryData
		if err := rows.Scan(&e.Phrase, &e.Canonical, &e.Category); err != nil {
			return entries
		}
		entries = append(entries, e)
	}
	return entries
}

// --- SQLite TaxonomyView ---

type sqliteTaxonomyView struct{ db *sql.DB }

func (v *sqliteTaxonomyView) CategoriesForToken(token string) []string {
	lowerToken := strings.ToLower(token)
	var cats []string
	for _, q := range []string{
		`SELECT DISTINCT name FROM taxonomy_sectors WHERE LOWER(keyword)=?`,
		`SELECT DISTINCT name FROM taxonomy_events WHERE LOWER(keyword)=?`,
		`SELECT DISTINCT name FROM taxonomy_regions WHERE LOWER(keyword)=?`,
	} {
		rows, err := v.db.Query(q, lowerToken)
		if err != nil {
			continue
		}
		for rows.Next() {
			var cat string
			if err := rows.Scan(&cat); err == nil {
				cats = append(cats, cat)
			}
		}
		rows.Close()
	}
	return cats
}

func (v *sqliteTaxonomyView) EntitiesInText(text string) []store.Entity {
	lowerText := strings.ToLower(text)
	rows, err := v.db.Query(`SELECT type, name, keyword FROM taxonomy_entities`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var ents []store.Entity
	for rows.Next() {
		var typ, name, keyword string
		if err := rows.Scan(&typ, &name, &keyword); err != nil {
			continue
		}
		key := typ + "|" + name
		if _, done := seen[key]; done {
			continue
		}
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			seen[key] = struct{}{}
			ents = append(ents, store.Entity{Type: typ, Value: name})
		}
	}
	return ents
}

func (v *sqliteTaxonomyView) AllSectors() map[string][]string {
	return v.loadCategoryMap(`SELECT name, keyword FROM taxonomy_sectors`)
}

func (v *sqliteTaxonomyView) AllEvents() map[string][]string {
	return v.loadCategoryMap(`SELECT name, keyword FROM taxonomy_events`)
}

func (v *sqliteTaxonomyView) AllRegions() map[string][]string {
	return v.loadCategoryMap(`SELECT name, keyword FROM taxonomy_regions`)
}

func (v *sqliteTaxonomyView) loadCategoryMap(query string) map[string][]string {
	rows, err := v.db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	m := make(map[string][]string)
	for rows.Next() {
		var name, keyword string
		if err := rows.Scan(&name, &keyword); err == nil {
			m[name] = append(m[name], keyword)
		}
	}
	return m
}

func (v *sqliteTaxonomyView) AllEntities() map[string]map[string][]string {
	rows, err := v.db.Query(`SELECT type, name, keyword FROM taxonomy_entities`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	m := make(map[string]map[string][]string)
	for rows.Next() {
		var typ, name, keyword string
		if err := rows.Scan(&typ, &name, &keyword); err == nil {
			if m[typ] == nil {
				m[typ] = make(map[string][]string)
			}
			m[typ][name] = append(m[typ][name], keyword)
		}
	}
	return m
}
