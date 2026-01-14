package memstore

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cognicore/korel/pkg/korel/store"
)

// Store is an in-memory implementation of store.Store for tests.
type Store struct {
	mu         sync.RWMutex
	nextID     int64
	docs       map[int64]store.Doc
	urlIndex   map[string]int64
	tokenDF    map[string]int64
	pairCounts map[string]int64
	cards      map[string]store.Card
}

// New creates a new in-memory store.
func New() *Store {
	return &Store{
		nextID:     1,
		docs:       make(map[int64]store.Doc),
		urlIndex:   make(map[string]int64),
		tokenDF:    make(map[string]int64),
		pairCounts: make(map[string]int64),
		cards:      make(map[string]store.Card),
	}
}

// Close implements store.Store.
func (s *Store) Close() error { return nil }

// UpsertDoc inserts or updates a document, keyed by URL.
func (s *Store) UpsertDoc(ctx context.Context, d store.Doc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.URL == "" {
		return nil
	}

	var id int64
	if existingID, ok := s.urlIndex[d.URL]; ok {
		id = existingID
	} else {
		id = s.nextID
		s.nextID++
		d.ID = id
		s.urlIndex[d.URL] = id
	}

	d.ID = id
	s.docs[id] = copyDoc(d)
	return nil
}

// GetDoc returns a document by ID.
func (s *Store) GetDoc(ctx context.Context, id int64) (store.Doc, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if doc, ok := s.docs[id]; ok {
		return copyDoc(doc), nil
	}
	return store.Doc{}, nil
}

// GetDocByURL returns a document by URL.
func (s *Store) GetDocByURL(ctx context.Context, url string) (store.Doc, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id, ok := s.urlIndex[url]; ok {
		if doc, exists := s.docs[id]; exists {
			return copyDoc(doc), true, nil
		}
	}
	return store.Doc{}, false, nil
}

// GetDocsByTokens returns documents that contain any of the provided tokens.
func (s *Store) GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]store.Doc, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	tokenSet := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		tokenSet[tok] = struct{}{}
	}

	type scored struct {
		doc store.Doc
		ts  time.Time
	}

	var results []scored
	for _, doc := range s.docs {
		if containsAny(doc.Tokens, tokenSet) {
			results = append(results, scored{doc: copyDoc(doc), ts: doc.PublishedAt})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ts.After(results[j].ts)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]store.Doc, len(results))
	for i, res := range results {
		out[i] = res.doc
	}
	return out, nil
}

// UpsertTokenDF sets the document frequency for a token.
func (s *Store) UpsertTokenDF(ctx context.Context, token string, df int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if df < 0 {
		df = 0
	}
	if token == "" {
		return nil
	}
	if df == 0 {
		delete(s.tokenDF, token)
		return nil
	}
	s.tokenDF[token] = df
	return nil
}

// GetTokenDF returns the document frequency for a token.
func (s *Store) GetTokenDF(ctx context.Context, token string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokenDF[token], nil
}

// IncPair increments the co-occurrence count for a pair.
func (s *Store) IncPair(ctx context.Context, t1, t2 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := pairKey(t1, t2)
	if key == "" {
		return nil
	}
	s.pairCounts[key]++
	return nil
}

// DecPair decrements the co-occurrence count for a pair.
func (s *Store) DecPair(ctx context.Context, t1, t2 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := pairKey(t1, t2)
	if key == "" {
		return nil
	}
	if s.pairCounts[key] <= 1 {
		delete(s.pairCounts, key)
	} else {
		s.pairCounts[key]--
	}
	return nil
}

// GetPMI returns the PMI value for a pair if present.
func (s *Store) GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := pairKey(t1, t2)
	if key == "" {
		return 0, false, nil
	}

	count, ok := s.pairCounts[key]
	if !ok {
		return 0, false, nil
	}

	dfA := s.tokenDF[t1]
	dfB := s.tokenDF[t2]
	total := int64(len(s.docs))
	if total == 0 {
		return 0, false, nil
	}

	epsilon := 1.0
	numerator := (float64(count) + epsilon) * float64(total)
	denominator := (float64(dfA) + epsilon) * (float64(dfB) + epsilon)
	if denominator == 0 {
		return 0, false, nil
	}
	return math.Log(numerator / denominator), true, nil
}

// TopNeighbors returns top pairs by raw count.
func (s *Store) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if k <= 0 {
		k = 10
	}

	type neighborCount struct {
		token string
		count int64
	}

	var candidates []neighborCount
	for key, count := range s.pairCounts {
		tokens := strings.Split(key, "|")
		if len(tokens) != 2 {
			continue
		}
		if tokens[0] == token {
			candidates = append(candidates, neighborCount{token: tokens[1], count: count})
		} else if tokens[1] == token {
			candidates = append(candidates, neighborCount{token: tokens[0], count: count})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})
	if len(candidates) > k {
		candidates = candidates[:k]
	}

	neighbors := make([]store.Neighbor, len(candidates))
	for i, cand := range candidates {
		neighbors[i] = store.Neighbor{
			Token: cand.token,
			PMI:   float64(cand.count),
		}
	}
	return neighbors, nil
}

// UpsertCard stores a card in memory.
func (s *Store) UpsertCard(ctx context.Context, c store.Card) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = time.Now().Format(time.RFC3339Nano)
	}
	s.cards[c.ID] = c
	return nil
}

// GetCardsByPeriod returns cards for a period.
func (s *Store) GetCardsByPeriod(ctx context.Context, period string, k int) ([]store.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []store.Card
	for _, card := range s.cards {
		if card.Period == period {
			result = append(result, card)
		}
	}
	if len(result) > k && k > 0 {
		result = result[:k]
	}
	return result, nil
}

// Stoplist returns nil (not implemented).
func (s *Store) Stoplist() store.StoplistView { return nil }

// Dict returns nil (not implemented).
func (s *Store) Dict() store.DictView { return nil }

// Taxonomy returns nil (not implemented).
func (s *Store) Taxonomy() store.TaxonomyView { return nil }

func containsAny(tokens []string, set map[string]struct{}) bool {
	for _, tok := range tokens {
		if _, ok := set[tok]; ok {
			return true
		}
	}
	return false
}

func copyDoc(d store.Doc) store.Doc {
	copySlice := func(in []string) []string {
		out := make([]string, len(in))
		copy(out, in)
		return out
	}

	copyEntities := func(in []store.Entity) []store.Entity {
		out := make([]store.Entity, len(in))
		copy(out, in)
		return out
	}

	return store.Doc{
		ID:          d.ID,
		URL:         d.URL,
		Title:       d.Title,
		Outlet:      d.Outlet,
		PublishedAt: d.PublishedAt,
		Cats:        copySlice(d.Cats),
		Ents:        copyEntities(d.Ents),
		LinksOut:    d.LinksOut,
		Tokens:      copySlice(d.Tokens),
	}
}

func pairKey(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	if a == b {
		return ""
	}
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}
