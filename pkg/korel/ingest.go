package korel

import (
	"context"
	"time"

	"github.com/cognicore/korel/pkg/korel/cards"
	"github.com/cognicore/korel/pkg/korel/store"
)

// IngestDoc represents a document to be ingested
type IngestDoc struct {
	URL         string
	Title       string
	Outlet      string
	PublishedAt time.Time
	BodyText    string
	SourceCats  []string
}

// Ingest processes and stores a document
func (k *Korel) Ingest(ctx context.Context, d IngestDoc) error {
	existingDoc, found, err := k.store.GetDocByURL(ctx, d.URL)
	if err != nil {
		return err
	}

	// Process through pipeline
	processed := k.pipeline.Process(d.BodyText)

	// Convert ingest.Entity to store.Entity
	storeEntities := make([]store.Entity, len(processed.Entities))
	for i, e := range processed.Entities {
		storeEntities[i] = store.Entity{
			Type:  e.Type,
			Value: e.Value,
		}
	}

	// Store document
	doc := store.Doc{
		URL:         d.URL,
		Title:       d.Title,
		BodySnippet: cards.ExtractSnippet(d.BodyText),
		Outlet:      d.Outlet,
		PublishedAt: d.PublishedAt,
		Cats:        uniqueStrings(append(d.SourceCats, processed.Categories...)),
		Ents:        storeEntities,
		Tokens:      processed.Tokens,
	}

	if err := k.store.UpsertDoc(ctx, doc); err != nil {
		return err
	}

	if found {
		if err := k.updateStats(ctx, existingDoc.Tokens, -1); err != nil {
			return err
		}
	}
	if err := k.updateStats(ctx, processed.Tokens, 1); err != nil {
		return err
	}

	return nil
}

// batchPairStore is an optional interface for stores that support batch pair operations.
type batchPairStore interface {
	BatchIncPairs(pairs [][2]string)
	BatchDecPairs(pairs [][2]string)
}

func (k *Korel) updateStats(ctx context.Context, tokens []string, delta int) error {
	if delta == 0 {
		return nil
	}
	unique := uniqueStrings(tokens)
	if len(unique) == 0 {
		return nil
	}

	for _, tok := range unique {
		current, err := k.store.GetTokenDF(ctx, tok)
		if err != nil {
			return err
		}
		newVal := current + int64(delta)
		if newVal < 0 {
			newVal = 0
		}
		if err := k.store.UpsertTokenDF(ctx, tok, newVal); err != nil {
			return err
		}
	}

	// Collect all pairs, then batch-write under a single lock if supported.
	n := len(unique)
	pairs := make([][2]string, 0, n*(n-1)/2)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			pairs = append(pairs, [2]string{unique[i], unique[j]})
		}
	}

	if bs, ok := k.store.(batchPairStore); ok {
		if delta > 0 {
			bs.BatchIncPairs(pairs)
		} else {
			bs.BatchDecPairs(pairs)
		}
	} else {
		for _, p := range pairs {
			var err error
			if delta > 0 {
				err = k.store.IncPair(ctx, p[0], p[1])
			} else {
				err = k.store.DecPair(ctx, p[0], p[1])
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}
