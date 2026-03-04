package sqlite

import (
	"context"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// RecordFeedback inserts a feedback event.
func (s *sqliteStore) RecordFeedback(ctx context.Context, sessionID, queryHash, cardID, action string, ts time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feedback (session_id, query_hash, card_id, action, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, queryHash, cardID, action, ts.UTC().Format(time.RFC3339))
	return err
}

// GetFeedbackStats returns aggregated feedback statistics.
func (s *sqliteStore) GetFeedbackStats(ctx context.Context) (store.FeedbackStats, error) {
	var stats store.FeedbackStats

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM feedback WHERE action = 'click'`).Scan(&stats.TotalClicks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM feedback WHERE action = 'dismiss'`).Scan(&stats.TotalDismisses)
	if err != nil {
		return stats, err
	}

	return stats, nil
}
