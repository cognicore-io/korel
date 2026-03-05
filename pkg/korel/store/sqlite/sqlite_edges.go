package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/cognicore/korel/pkg/korel/store"
)

// UpsertEdge inserts or updates an edge in the knowledge graph.
func (s *sqliteStore) UpsertEdge(ctx context.Context, e store.Edge) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO edges (subject, relation, object, weight, source)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(subject, relation, object) DO UPDATE SET
	weight=excluded.weight, source=excluded.source;
`, e.Subject, e.Relation, e.Object, e.Weight, e.Source)
	return err
}

// GetEdges returns all edges with the given subject.
func (s *sqliteStore) GetEdges(ctx context.Context, subject string) ([]store.Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT subject, relation, object, weight, source FROM edges WHERE subject = ?`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// GetEdgesByRelation returns edges of a given relation type.
func (s *sqliteStore) GetEdgesByRelation(ctx context.Context, relation string, limit int) ([]store.Edge, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT subject, relation, object, weight, source FROM edges WHERE relation = ? LIMIT ?`,
		relation, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// DeleteEdgesBySource removes all edges from a given source.
func (s *sqliteStore) DeleteEdgesBySource(ctx context.Context, source string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM edges WHERE source = ?`, source)
	return err
}

// AllEdges returns every edge in the knowledge graph.
func (s *sqliteStore) AllEdges(ctx context.Context) ([]store.Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT subject, relation, object, weight, source FROM edges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

func scanEdges(rows *sql.Rows) ([]store.Edge, error) {
	var edges []store.Edge
	for rows.Next() {
		var e store.Edge
		if err := rows.Scan(&e.Subject, &e.Relation, &e.Object, &e.Weight, &e.Source); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
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
