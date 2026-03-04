package korel

import (
	"context"
	"time"
)

// EntityTimeline is a chronological sequence of entity mentions across documents.
type EntityTimeline struct {
	Entity string
	Type   string // TICKER, COUNTRY, etc.
	Events []EntityEvent
}

// EntityEvent represents a single entity mention in a document.
type EntityEvent struct {
	DocID   int64
	Title   string
	Time    time.Time
	Snippet string
}

// BuildEntityTimeline queries documents mentioning an entity, ordered by time.
// If entityType is empty, matches any type. Results are capped at limit.
func (k *Korel) BuildEntityTimeline(ctx context.Context, entityType, entityValue string, limit int) (EntityTimeline, error) {
	if limit <= 0 {
		limit = 20
	}

	docs, err := k.store.GetDocsByEntity(ctx, entityType, entityValue, limit)
	if err != nil {
		return EntityTimeline{}, err
	}

	tl := EntityTimeline{
		Entity: entityValue,
		Type:   entityType,
		Events: make([]EntityEvent, 0, len(docs)),
	}

	for _, doc := range docs {
		tl.Events = append(tl.Events, EntityEvent{
			DocID:   doc.ID,
			Title:   doc.Title,
			Time:    doc.PublishedAt,
			Snippet: doc.BodySnippet,
		})
	}

	return tl, nil
}
