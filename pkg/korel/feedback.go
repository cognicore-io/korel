package korel

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// Feedback records a user interaction with a search result.
type Feedback struct {
	SessionID string
	QueryHash string
	CardID    string
	Action    FeedbackAction
	Timestamp time.Time
}

// FeedbackAction is the type of user interaction.
type FeedbackAction string

const (
	FeedbackClick   FeedbackAction = "click"
	FeedbackDismiss FeedbackAction = "dismiss"
)

// RecordFeedback persists a feedback event.
func (k *Korel) RecordFeedback(ctx context.Context, fb Feedback) error {
	if fb.Timestamp.IsZero() {
		fb.Timestamp = time.Now()
	}
	return k.store.RecordFeedback(ctx, fb.SessionID, fb.QueryHash, fb.CardID, string(fb.Action), fb.Timestamp)
}

// GetFeedbackStats returns aggregated feedback statistics for adaptive ranking.
func (k *Korel) GetFeedbackStats(ctx context.Context) (store.FeedbackStats, error) {
	return k.store.GetFeedbackStats(ctx)
}

// QueryHash produces a stable hash for a query string (used as feedback key).
func QueryHash(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%x", h[:8])
}
