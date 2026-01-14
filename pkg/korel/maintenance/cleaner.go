package maintenance

import (
	"context"
	"errors"

	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store"
)

// DocSource abstracts how we iterate documents for cleaning.
type DocSource interface {
	Next(ctx context.Context) (store.Doc, bool, error)
}

// Cleaner reprocesses documents after stoplist/taxonomy updates.
type Cleaner struct {
	Store    store.Store
	Pipeline *ingest.Pipeline
	Source   DocSource
}

// Result summarizes the cleaning run.
type Result struct {
	Processed int
	Updated   int
	Errors    int
}

// Clean replays docs from the source, removing tokens that no longer pass the pipeline.
func (c *Cleaner) Clean(ctx context.Context) (Result, error) {
	var res Result
	if c.Store == nil || c.Pipeline == nil || c.Source == nil {
		return res, errors.New("cleaner: invalid configuration")
	}

	for {
		doc, ok, err := c.Source.Next(ctx)
		if err != nil {
			res.Errors++
			continue
		}
		if !ok {
			break
		}
		res.Processed++

		processed := c.Pipeline.Process(doc.Title + " " + doc.URL) // placeholder: real impl should use body
		if slicesEqual(processed.Tokens, doc.Tokens) {
			continue
		}

		doc.Tokens = processed.Tokens
		doc.Cats = processed.Categories
		if err := c.Store.UpsertDoc(ctx, doc); err != nil {
			res.Errors++
			continue
		}
		res.Updated++
	}
	return res, nil
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
