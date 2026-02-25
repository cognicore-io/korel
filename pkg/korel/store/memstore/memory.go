package memstore

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store"
)

// ipair is a sorted integer pair key for O(1) hashing.
// Tokens are interned to int32 IDs; the pair is always stored with A < B.
type ipair [2]int32

// Store is an in-memory implementation of store.Store for tests.
type Store struct {
	mu         sync.RWMutex
	nextID     int64
	docs       map[int64]store.Doc
	urlIndex   map[string]int64
	tokenDF    map[string]int64
	pairCounts map[ipair]int64
	intern     map[string]int32 // token string → integer ID
	internRev  []string         // integer ID → token string
	neighbors  map[int32][]int32 // lazy: built on first TopNeighbors call (integer-keyed)
	nbDirty    bool              // true when pairCounts changed since last neighbor rebuild
	cards      map[string]store.Card
	pmiCfg     pmi.Config
	calc       *pmi.Calculator
	stops      map[string]struct{}              // stopword set
	dict       map[string]dictEntry             // phrase → {canonical, category}
	taxSectors map[string][]string              // sector name → keywords
	taxEvents  map[string][]string              // event name → keywords
	taxRegions map[string][]string              // region name → keywords
	taxEnts    map[string]map[string][]string   // entity type → name → keywords
}

type dictEntry struct {
	canonical string
	category  string
}

// New creates a new in-memory store.
// An optional pmi.Config can be passed to control PMI computation;
// if omitted, pmi.DefaultConfig() is used.
func New(cfg ...pmi.Config) *Store {
	c := pmi.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Store{
		nextID:     1,
		docs:       make(map[int64]store.Doc),
		urlIndex:   make(map[string]int64),
		tokenDF:    make(map[string]int64),
		pairCounts: make(map[ipair]int64, 1024),
		intern:     make(map[string]int32, 512),
		nbDirty:    true,
		cards:      make(map[string]store.Card),
		pmiCfg:     c,
		calc:       pmi.NewCalculatorFromConfig(c),
	}
}

// internToken returns the integer ID for a token, assigning a new one if needed.
// Caller must hold s.mu write lock.
func (s *Store) internToken(tok string) int32 {
	if id, ok := s.intern[tok]; ok {
		return id
	}
	id := int32(len(s.internRev))
	s.intern[tok] = id
	s.internRev = append(s.internRev, tok)
	return id
}

// makeIPair creates a sorted integer pair from two token strings.
// Returns the pair and false if either token is empty or they are equal.
// Caller must hold s.mu write lock (for internToken).
func (s *Store) makeIPair(t1, t2 string) (ipair, bool) {
	if t1 == "" || t2 == "" || t1 == t2 {
		return ipair{}, false
	}
	a := s.internToken(t1)
	b := s.internToken(t2)
	if a > b {
		a, b = b, a
	}
	return ipair{a, b}, true
}

// lookupIPair creates a sorted integer pair without interning new tokens.
// Returns the pair and false if either token is not interned or they are equal.
// Caller must hold s.mu read lock.
func (s *Store) lookupIPair(t1, t2 string) (ipair, bool) {
	if t1 == "" || t2 == "" || t1 == t2 {
		return ipair{}, false
	}
	a, okA := s.intern[t1]
	b, okB := s.intern[t2]
	if !okA || !okB {
		return ipair{}, false
	}
	if a > b {
		a, b = b, a
	}
	return ipair{a, b}, true
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
	s.incPairLocked(t1, t2)
	return nil
}

// incPairLocked increments a pair count without acquiring the lock.
func (s *Store) incPairLocked(t1, t2 string) {
	p, ok := s.makeIPair(t1, t2)
	if !ok {
		return
	}
	s.pairCounts[p]++
	s.nbDirty = true
}

// DecPair decrements the co-occurrence count for a pair.
func (s *Store) DecPair(ctx context.Context, t1, t2 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decPairLocked(t1, t2)
	return nil
}

// decPairLocked decrements a pair count without acquiring the lock.
func (s *Store) decPairLocked(t1, t2 string) {
	p, ok := s.makeIPair(t1, t2)
	if !ok {
		return
	}
	if s.pairCounts[p] <= 1 {
		delete(s.pairCounts, p)
	} else {
		s.pairCounts[p]--
	}
	s.nbDirty = true
}

// BatchIncPairs increments counts for multiple pairs under a single lock.
// Pre-interns all unique tokens once, then operates with integer pairs only.
func (s *Store) BatchIncPairs(pairs [][2]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Pre-intern all unique tokens in a single pass.
	cache := make(map[string]int32, 256)
	for _, p := range pairs {
		if _, ok := cache[p[0]]; !ok {
			cache[p[0]] = s.internToken(p[0])
		}
		if _, ok := cache[p[1]]; !ok {
			cache[p[1]] = s.internToken(p[1])
		}
	}

	// Increment using pre-interned IDs.
	for _, p := range pairs {
		if p[0] == "" || p[1] == "" || p[0] == p[1] {
			continue
		}
		a, b := cache[p[0]], cache[p[1]]
		if a > b {
			a, b = b, a
		}
		s.pairCounts[ipair{a, b}]++
	}
	s.nbDirty = true
}

// BatchDecPairs decrements counts for multiple pairs under a single lock.
// Pre-interns all unique tokens once, then operates with integer pairs only.
func (s *Store) BatchDecPairs(pairs [][2]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Pre-intern all unique tokens in a single pass.
	cache := make(map[string]int32, 256)
	for _, p := range pairs {
		if _, ok := cache[p[0]]; !ok {
			cache[p[0]] = s.internToken(p[0])
		}
		if _, ok := cache[p[1]]; !ok {
			cache[p[1]] = s.internToken(p[1])
		}
	}

	// Decrement using pre-interned IDs.
	for _, p := range pairs {
		if p[0] == "" || p[1] == "" || p[0] == p[1] {
			continue
		}
		a, b := cache[p[0]], cache[p[1]]
		if a > b {
			a, b = b, a
		}
		ip := ipair{a, b}
		if s.pairCounts[ip] <= 1 {
			delete(s.pairCounts, ip)
		} else {
			s.pairCounts[ip]--
		}
	}
	s.nbDirty = true
}

// GetPMI returns the PMI value for a pair if present.
func (s *Store) GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.lookupIPair(t1, t2)
	if !ok {
		return 0, false, nil
	}

	count, exists := s.pairCounts[p]
	if !exists {
		return 0, false, nil
	}

	dfA := s.tokenDF[t1]
	dfB := s.tokenDF[t2]
	total := int64(len(s.docs))
	if total == 0 {
		return 0, false, nil
	}

	score := s.calc.Score(count, dfA, dfB, total, s.pmiCfg.UseNPMI)
	return score, true, nil
}

// TopNeighbors returns the top K neighbors ranked by PMI score.
// Uses a lazy adjacency index — rebuilt from pairCounts on first call after mutations.
func (s *Store) TopNeighbors(ctx context.Context, token string, k int) ([]store.Neighbor, error) {
	s.mu.Lock()
	s.rebuildNeighborsLocked()
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if k <= 0 {
		k = 10
	}

	total := int64(len(s.docs))
	if total == 0 {
		return nil, nil
	}

	dfToken := s.tokenDF[token]
	if dfToken == 0 {
		return nil, nil
	}

	tokenID, ok := s.intern[token]
	if !ok {
		return nil, nil
	}

	nbIDs := s.neighbors[tokenID]
	if len(nbIDs) == 0 {
		return nil, nil
	}

	neighbors := make([]store.Neighbor, 0, len(nbIDs))
	for _, otherID := range nbIDs {
		other := s.internRev[otherID]
		dfOther := s.tokenDF[other]
		if dfOther < s.pmiCfg.MinDF {
			continue
		}

		a, b := tokenID, otherID
		if a > b {
			a, b = b, a
		}
		count := s.pairCounts[ipair{a, b}]
		score := s.calc.Score(count, dfToken, dfOther, total, s.pmiCfg.UseNPMI)

		neighbors = append(neighbors, store.Neighbor{
			Token: other,
			PMI:   score,
		})
	}

	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].PMI > neighbors[j].PMI
	})
	if len(neighbors) > k {
		neighbors = neighbors[:k]
	}

	return neighbors, nil
}

// rebuildNeighborsLocked rebuilds the adjacency index from pairCounts.
// Uses integer keys to avoid string hashing entirely during rebuild.
// Caller must hold s.mu write lock.
func (s *Store) rebuildNeighborsLocked() {
	if !s.nbDirty {
		return
	}
	nb := make(map[int32][]int32, len(s.internRev))
	for p := range s.pairCounts {
		nb[p[0]] = append(nb[p[0]], p[1])
		nb[p[1]] = append(nb[p[1]], p[0])
	}
	s.neighbors = nb
	s.nbDirty = false
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

// Stoplist returns a read-only view of the stopword set, or nil if not configured.
func (s *Store) Stoplist() store.StoplistView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stops == nil {
		return nil
	}
	return &memStoplistView{store: s}
}

// Dict returns a read-only view of the multi-token dictionary, or nil if not configured.
func (s *Store) Dict() store.DictView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dict == nil {
		return nil
	}
	return &memDictView{store: s}
}

// Taxonomy returns a read-only view of the taxonomy, or nil if not configured.
func (s *Store) Taxonomy() store.TaxonomyView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.taxSectors == nil && s.taxEvents == nil && s.taxRegions == nil && s.taxEnts == nil {
		return nil
	}
	return &memTaxonomyView{store: s}
}

// UpsertStoplist replaces the stopword set (implements store.Store).
func (s *Store) UpsertStoplist(ctx context.Context, tokens []string) error {
	s.SetStoplist(tokens)
	return nil
}

// UpsertDictEntry adds or replaces a dictionary entry (implements store.Store).
func (s *Store) UpsertDictEntry(ctx context.Context, phrase, canonical, category string) error {
	s.AddDictEntry(phrase, canonical, category)
	return nil
}

// --- Mutators (memstore-specific convenience methods) ---

// SetStoplist replaces the stopword set.
func (s *Store) SetStoplist(tokens []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stops = make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		s.stops[tok] = struct{}{}
	}
}

// AddDictEntry adds or replaces a dictionary entry.
func (s *Store) AddDictEntry(phrase, canonical, category string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dict == nil {
		s.dict = make(map[string]dictEntry)
	}
	s.dict[phrase] = dictEntry{canonical: canonical, category: category}
}

// SetTaxonomy replaces the full taxonomy.
func (s *Store) SetTaxonomy(sectors, events, regions map[string][]string, entities map[string]map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taxSectors = sectors
	s.taxEvents = events
	s.taxRegions = regions
	s.taxEnts = entities
}

// --- StoplistView ---

type memStoplistView struct{ store *Store }

func (v *memStoplistView) IsStop(token string) bool {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	_, ok := v.store.stops[token]
	return ok
}

func (v *memStoplistView) AllStops() []string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	out := make([]string, 0, len(v.store.stops))
	for tok := range v.store.stops {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

// --- DictView ---

type memDictView struct{ store *Store }

func (v *memDictView) Lookup(phrase string) (canonical string, category string, ok bool) {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	e, found := v.store.dict[phrase]
	if !found {
		return "", "", false
	}
	return e.canonical, e.category, true
}

func (v *memDictView) AllEntries() []store.DictEntryData {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	entries := make([]store.DictEntryData, 0, len(v.store.dict))
	for phrase, e := range v.store.dict {
		entries = append(entries, store.DictEntryData{
			Phrase:    phrase,
			Canonical: e.canonical,
			Category:  e.category,
		})
	}
	return entries
}

// --- TaxonomyView ---

type memTaxonomyView struct{ store *Store }

func (v *memTaxonomyView) CategoriesForToken(token string) []string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	var cats []string
	lower := strings.ToLower(token)
	for name, keywords := range v.store.taxSectors {
		for _, kw := range keywords {
			if strings.ToLower(kw) == lower {
				cats = append(cats, name)
				break
			}
		}
	}
	for name, keywords := range v.store.taxEvents {
		for _, kw := range keywords {
			if strings.ToLower(kw) == lower {
				cats = append(cats, name)
				break
			}
		}
	}
	for name, keywords := range v.store.taxRegions {
		for _, kw := range keywords {
			if strings.ToLower(kw) == lower {
				cats = append(cats, name)
				break
			}
		}
	}
	return cats
}

func (v *memTaxonomyView) EntitiesInText(text string) []store.Entity {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	lower := strings.ToLower(text)
	var ents []store.Entity
	for entType, names := range v.store.taxEnts {
		for name, keywords := range names {
			for _, kw := range keywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					ents = append(ents, store.Entity{Type: entType, Value: name})
					break
				}
			}
		}
	}
	return ents
}

func (v *memTaxonomyView) AllSectors() map[string][]string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	return copyStringSliceMap(v.store.taxSectors)
}

func (v *memTaxonomyView) AllEvents() map[string][]string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	return copyStringSliceMap(v.store.taxEvents)
}

func (v *memTaxonomyView) AllRegions() map[string][]string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	return copyStringSliceMap(v.store.taxRegions)
}

func (v *memTaxonomyView) AllEntities() map[string]map[string][]string {
	v.store.mu.RLock()
	defer v.store.mu.RUnlock()
	out := make(map[string]map[string][]string, len(v.store.taxEnts))
	for typ, names := range v.store.taxEnts {
		nm := make(map[string][]string, len(names))
		for name, kws := range names {
			cp := make([]string, len(kws))
			copy(cp, kws)
			nm[name] = cp
		}
		out[typ] = nm
	}
	return out
}

func copyStringSliceMap(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

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
