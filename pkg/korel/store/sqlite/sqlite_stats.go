package sqlite

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/cognicore/korel/pkg/korel/store"
)

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

// GetTokenDFBatch retrieves document frequencies for multiple tokens in one query.
func (s *sqliteStore) GetTokenDFBatch(ctx context.Context, tokens []string) (map[string]int64, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	result := make(map[string]int64, len(tokens))

	// Process in chunks of 500 (SQLite variable limit)
	const chunkSize = 500
	for i := 0; i < len(tokens); i += chunkSize {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := tokens[i:end]

		ph := strings.Repeat("?,", len(chunk))
		ph = strings.TrimSuffix(ph, ",")
		args := make([]interface{}, len(chunk))
		for j, t := range chunk {
			args[j] = t
		}

		rows, err := s.db.QueryContext(ctx, `SELECT token, df FROM token_df WHERE token IN (`+ph+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var token string
			var df int64
			if err := rows.Scan(&token, &df); err != nil {
				rows.Close()
				return nil, err
			}
			result[token] = df
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return result, nil
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

// GetPairsBatch retrieves co-occurrence counts for all pairs where t1 is in queryTokens
// and t2 is in docTokens (or vice versa). Returns a map keyed by normalized (a,b) where a<b.
func (s *sqliteStore) GetPairsBatch(ctx context.Context, queryTokens, docTokens []string) (map[[2]string]int64, error) {
	if len(queryTokens) == 0 || len(docTokens) == 0 {
		return nil, nil
	}
	result := make(map[[2]string]int64)

	// Build query token set
	qSet := make(map[string]struct{}, len(queryTokens))
	for _, t := range queryTokens {
		qSet[t] = struct{}{}
	}

	// For each query token, fetch all its co-occurrence pairs in one query.
	// This is O(Q) queries instead of O(Q*D).
	for _, qt := range queryTokens {
		rows, err := s.db.QueryContext(ctx, `
			SELECT CASE WHEN t1 = ? THEN t2 ELSE t1 END AS other, count
			FROM token_pairs
			WHERE t1 = ? OR t2 = ?`, qt, qt, qt)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var other string
			var count int64
			if err := rows.Scan(&other, &count); err != nil {
				rows.Close()
				return nil, err
			}
			a, b := qt, other
			if a > b {
				a, b = b, a
			}
			result[[2]string{a, b}] = count
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// TopNeighbors returns the top K neighbors ranked by PMI score for a token.
// If k <= 0, all qualifying neighbors are returned (no limit).
func (s *sqliteStore) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {

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
	if k > 0 && len(neighbors) > k {
		neighbors = neighbors[:k]
	}
	return neighbors, nil
}

// AllTokens returns all distinct tokens in the corpus.
func (s *sqliteStore) AllTokens(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT token FROM doc_tokens`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}
