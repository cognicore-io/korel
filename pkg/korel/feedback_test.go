package korel

import (
	"context"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/store/memstore"
)

func TestRecordAndGetFeedback(t *testing.T) {
	ms := memstore.New()
	k := &Korel{store: ms}
	ctx := context.Background()

	// Record some feedback
	err := k.RecordFeedback(ctx, Feedback{
		SessionID: "sess-1",
		QueryHash: QueryHash("kubernetes"),
		CardID:    "card-1",
		Action:    FeedbackClick,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordFeedback: %v", err)
	}

	err = k.RecordFeedback(ctx, Feedback{
		SessionID: "sess-1",
		QueryHash: QueryHash("kubernetes"),
		CardID:    "card-2",
		Action:    FeedbackDismiss,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordFeedback: %v", err)
	}

	err = k.RecordFeedback(ctx, Feedback{
		SessionID: "sess-2",
		QueryHash: QueryHash("docker"),
		CardID:    "card-3",
		Action:    FeedbackClick,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordFeedback: %v", err)
	}

	// Check stats
	stats, err := k.GetFeedbackStats(ctx)
	if err != nil {
		t.Fatalf("GetFeedbackStats: %v", err)
	}
	if stats.TotalClicks != 2 {
		t.Errorf("expected 2 clicks, got %d", stats.TotalClicks)
	}
	if stats.TotalDismisses != 1 {
		t.Errorf("expected 1 dismiss, got %d", stats.TotalDismisses)
	}
}

func TestQueryHashDeterministic(t *testing.T) {
	h1 := QueryHash("kubernetes")
	h2 := QueryHash("kubernetes")
	if h1 != h2 {
		t.Errorf("same query should produce same hash: %q vs %q", h1, h2)
	}

	h3 := QueryHash("docker")
	if h1 == h3 {
		t.Errorf("different queries should produce different hashes: %q vs %q", h1, h3)
	}
}
