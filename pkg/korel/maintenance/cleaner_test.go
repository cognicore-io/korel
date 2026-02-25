package maintenance

import (
	"context"
	"errors"
	"testing"

	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store"
)

type fakeSource struct {
	docs []store.Doc
	idx  int
	err  error
}

func (f *fakeSource) Next(ctx context.Context) (store.Doc, bool, error) {
	if f.err != nil {
		return store.Doc{}, false, f.err
	}
	if f.idx >= len(f.docs) {
		return store.Doc{}, false, nil
	}
	doc := f.docs[f.idx]
	f.idx++
	return doc, true, nil
}

type fakeStore struct {
	docs []store.Doc
	err  error
}

func (f *fakeStore) Close() error { return nil }
func (f *fakeStore) UpsertDoc(ctx context.Context, d store.Doc) error {
	if f.err != nil {
		return f.err
	}
	f.docs = append(f.docs, d)
	return nil
}

func (f *fakeStore) GetDoc(ctx context.Context, id int64) (store.Doc, error)          { return store.Doc{}, nil }
func (f *fakeStore) GetDocByURL(ctx context.Context, url string) (store.Doc, bool, error) {
	return store.Doc{}, false, nil
}
func (f *fakeStore) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]store.Doc, error) {
	return nil, nil
}
func (f *fakeStore) UpsertTokenDF(ctx context.Context, token string, df int64) error { return nil }
func (f *fakeStore) GetTokenDF(ctx context.Context, token string) (int64, error)     { return 0, nil }
func (f *fakeStore) IncPair(ctx context.Context, t1, t2 string) error               { return nil }
func (f *fakeStore) DecPair(ctx context.Context, t1, t2 string) error               { return nil }
func (f *fakeStore) GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error) {
	return 0, false, nil
}
func (f *fakeStore) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {
	return nil, nil
}
func (f *fakeStore) UpsertCard(ctx context.Context, c store.Card) error          { return nil }
func (f *fakeStore) GetCardsByPeriod(ctx context.Context, period string, k int) ([]store.Card, error) {
	return nil, nil
}
func (f *fakeStore) Stoplist() store.StoplistView { return nil }
func (f *fakeStore) Dict() store.DictView         { return nil }
func (f *fakeStore) Taxonomy() store.TaxonomyView { return nil }
func (f *fakeStore) UpsertStoplist(ctx context.Context, tokens []string) error { return nil }
func (f *fakeStore) UpsertDictEntry(ctx context.Context, phrase, canonical, category string) error {
	return nil
}

func TestCleanerUpdatesTokens(t *testing.T) {
	tokenizer := ingest.NewTokenizer([]string{"the"})
	parser := ingest.NewMultiTokenParser([]ingest.DictEntry{})
	taxonomy := ingest.NewTaxonomy()
	pipeline := ingest.NewPipeline(tokenizer, parser, taxonomy)

	source := &fakeSource{
		docs: []store.Doc{
			{ID: 1, Title: "the quick brown", Tokens: []string{"the", "quick", "brown"}},
		},
	}
	st := &fakeStore{}

	cleaner := Cleaner{
		Store:    st,
		Pipeline: pipeline,
		Source:   source,
	}

	res, err := cleaner.Clean(context.Background())
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if res.Processed != 1 || res.Updated != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(st.docs) != 1 || len(st.docs[0].Tokens) != 2 {
		t.Fatalf("store doc not updated: %+v", st.docs)
	}
}

func TestCleanerHandlesStoreErrors(t *testing.T) {
	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer([]string{"foo"}),
		ingest.NewMultiTokenParser(nil),
		ingest.NewTaxonomy(),
	)

	cleaner := Cleaner{
		Store:    &fakeStore{err: errors.New("boom")},
		Pipeline: pipeline,
		Source: &fakeSource{
			docs: []store.Doc{{ID: 1, Title: "foo", Tokens: []string{"foo"}}},
		},
	}

	res, err := cleaner.Clean(context.Background())
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if res.Errors == 0 {
		t.Fatalf("expected error count, got %+v", res)
	}
}
