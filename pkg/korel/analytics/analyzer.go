package analytics

import (
	"context"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/signals"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

// Skip-gram window configuration constants
const (
	// DefaultSkipGramWindow is the default distance for c-token tracking.
	DefaultSkipGramWindow = 5

	// MinSkipGramWindow is the minimum valid window size.
	MinSkipGramWindow = 2
)

// ipair is a sorted integer pair key. Tokens are interned to int32 IDs.
type ipair [2]int32

// pair is the exported string pair type used in public APIs.
type pair struct {
	A string
	B string
}

func newPair(a, b string) pair {
	if a > b {
		a, b = b, a
	}
	return pair{A: a, B: b}
}

// Analyzer aggregates document-level token/category stats.
type Analyzer struct {
	totalDocs      int64
	tokenDF        map[string]int64
	tokenCats      map[string]map[string]int64
	pairCounts     map[ipair]int64 // document-level co-occurrence (interned keys)
	bigramCounts   map[ipair]int64 // adjacent token pairs only (interned, ordered)
	skipGramCounts map[ipair]int64 // window-based co-occurrence (interned, sorted)
	intern         map[string]int32
	internRev      []string
	windowSize     int
	pmiCfg         pmi.Config
	calc           *pmi.Calculator
	dampingCfg     *signals.DampingConfig
}

// internToken returns the integer ID for a token, assigning a new one if needed.
func (a *Analyzer) internToken(tok string) int32 {
	if id, ok := a.intern[tok]; ok {
		return id
	}
	id := int32(len(a.internRev))
	a.intern[tok] = id
	a.internRev = append(a.internRev, tok)
	return id
}

// NewAnalyzer creates an empty analyzer with default window size.
func NewAnalyzer(cfg ...pmi.Config) *Analyzer {
	c := pmi.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Analyzer{
		tokenDF:        make(map[string]int64, 512),
		tokenCats:      make(map[string]map[string]int64, 512),
		pairCounts:     make(map[ipair]int64, 4096),
		bigramCounts:   make(map[ipair]int64, 2048),
		skipGramCounts: make(map[ipair]int64, 2048),
		intern:         make(map[string]int32, 512),
		windowSize:     DefaultSkipGramWindow,
		pmiCfg:         c,
		calc:           pmi.NewCalculatorFromConfig(c),
	}
}

// WithDamping enables density-based damping on PMI scores.
func (a *Analyzer) WithDamping(cfg signals.DampingConfig) *Analyzer {
	a.dampingCfg = &cfg
	return a
}

// NewAnalyzerWithWindow creates an analyzer with a custom skip-gram window size.
func NewAnalyzerWithWindow(windowSize int) *Analyzer {
	a := NewAnalyzer()
	if windowSize < MinSkipGramWindow {
		windowSize = MinSkipGramWindow
	}
	a.windowSize = windowSize
	return a
}

// Process consumes one document's tokens/categories.
func (a *Analyzer) Process(tokens []string, categories []string) {
	a.totalDocs++

	seen := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		a.tokenDF[tok]++
		for _, cat := range categories {
			if cat == "" {
				continue
			}
			if a.tokenCats[tok] == nil {
				a.tokenCats[tok] = make(map[string]int64)
			}
			a.tokenCats[tok][cat]++
		}
	}

	// Document-level pair counts using interned integer IDs.
	ids := make([]int32, 0, len(seen))
	for tok := range seen {
		ids = append(ids, a.internToken(tok))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a.pairCounts[ipair{ids[i], ids[j]}]++
		}
	}

	// Bigram counts (adjacent tokens only, preserving order)
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i] == "" || tokens[i+1] == "" {
			continue
		}
		idA := a.internToken(tokens[i])
		idB := a.internToken(tokens[i+1])
		a.bigramCounts[ipair{idA, idB}]++ // ordered, not sorted
	}

	// Skip-gram counts (window-based, deduplicated per document)
	skipGramSeen := make(map[ipair]struct{})
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "" {
			continue
		}
		idI := a.internToken(tokens[i])
		for j := i + 1; j < len(tokens) && j < i+a.windowSize; j++ {
			if tokens[j] == "" || tokens[j] == tokens[i] {
				continue
			}
			idJ := a.internToken(tokens[j])
			// Sorted pair
			p := ipair{idI, idJ}
			if idI > idJ {
				p = ipair{idJ, idI}
			}
			skipGramSeen[p] = struct{}{}
		}
	}
	for p := range skipGramSeen {
		a.skipGramCounts[p]++
	}
}

// DocTokens holds pre-processed document data for batch processing.
type DocTokens struct {
	Tokens     []string
	Categories []string
}

// localCounts holds per-goroutine counts accumulated during ProcessBatch.
type localCounts struct {
	tokenDF        map[string]int64
	tokenCats      map[string]map[string]int64
	pairCounts     map[ipair]int64
	bigramCounts   map[ipair]int64
	skipGramCounts map[ipair]int64
	totalDocs      int64
	intern         map[string]int32
	internRev      []string
}

func newLocalCounts() *localCounts {
	return &localCounts{
		tokenDF:        make(map[string]int64, 512),
		tokenCats:      make(map[string]map[string]int64, 128),
		pairCounts:     make(map[ipair]int64, 4096),
		bigramCounts:   make(map[ipair]int64, 2048),
		skipGramCounts: make(map[ipair]int64, 2048),
		intern:         make(map[string]int32, 512),
	}
}

func (lc *localCounts) internToken(tok string) int32 {
	if id, ok := lc.intern[tok]; ok {
		return id
	}
	id := int32(len(lc.internRev))
	lc.intern[tok] = id
	lc.internRev = append(lc.internRev, tok)
	return id
}

// ProcessBatch processes multiple documents in parallel, then merges results.
func (a *Analyzer) ProcessBatch(docs []DocTokens) {
	nWorkers := runtime.GOMAXPROCS(0)
	if nWorkers > len(docs) {
		nWorkers = len(docs)
	}
	if nWorkers <= 1 {
		for _, doc := range docs {
			a.Process(doc.Tokens, doc.Categories)
		}
		return
	}

	locals := make([]*localCounts, nWorkers)
	var wg sync.WaitGroup
	chunkSize := (len(docs) + nWorkers - 1) / nWorkers

	for w := 0; w < nWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(docs) {
			end = len(docs)
		}
		if start >= end {
			break
		}
		lc := newLocalCounts()
		locals[w] = lc
		wg.Add(1)
		go func(chunk []DocTokens, lc *localCounts) {
			defer wg.Done()
			windowSize := a.windowSize
			for _, doc := range chunk {
				lc.totalDocs++
				tokens := doc.Tokens
				categories := doc.Categories

				seen := make(map[string]struct{}, len(tokens))
				for _, tok := range tokens {
					if tok == "" {
						continue
					}
					if _, ok := seen[tok]; ok {
						continue
					}
					seen[tok] = struct{}{}
					lc.tokenDF[tok]++
					for _, cat := range categories {
						if cat == "" {
							continue
						}
						if lc.tokenCats[tok] == nil {
							lc.tokenCats[tok] = make(map[string]int64)
						}
						lc.tokenCats[tok][cat]++
					}
				}

				ids := make([]int32, 0, len(seen))
				for tok := range seen {
					ids = append(ids, lc.internToken(tok))
				}
				sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
				for i := 0; i < len(ids); i++ {
					for j := i + 1; j < len(ids); j++ {
						lc.pairCounts[ipair{ids[i], ids[j]}]++
					}
				}

				for i := 0; i < len(tokens)-1; i++ {
					if tokens[i] == "" || tokens[i+1] == "" {
						continue
					}
					idA := lc.internToken(tokens[i])
					idB := lc.internToken(tokens[i+1])
					lc.bigramCounts[ipair{idA, idB}]++
				}

				skipGramSeen := make(map[ipair]struct{})
				for i := 0; i < len(tokens); i++ {
					if tokens[i] == "" {
						continue
					}
					idI := lc.internToken(tokens[i])
					for j := i + 1; j < len(tokens) && j < i+windowSize; j++ {
						if tokens[j] == "" || tokens[j] == tokens[i] {
							continue
						}
						idJ := lc.internToken(tokens[j])
						p := ipair{idI, idJ}
						if idI > idJ {
							p = ipair{idJ, idI}
						}
						skipGramSeen[p] = struct{}{}
					}
				}
				for p := range skipGramSeen {
					lc.skipGramCounts[p]++
				}
			}
		}(docs[start:end], lc)
	}
	wg.Wait()

	// Merge local counts into the analyzer.
	for _, lc := range locals {
		if lc == nil {
			continue
		}
		a.totalDocs += lc.totalDocs

		for tok, df := range lc.tokenDF {
			a.tokenDF[tok] += df
		}
		for tok, cats := range lc.tokenCats {
			if a.tokenCats[tok] == nil {
				a.tokenCats[tok] = make(map[string]int64, len(cats))
			}
			for cat, count := range cats {
				a.tokenCats[tok][cat] += count
			}
		}

		// Re-intern local tokens to global IDs and merge pair counts.
		remap := make([]int32, len(lc.internRev))
		for i, tok := range lc.internRev {
			remap[i] = a.internToken(tok)
		}
		for p, count := range lc.pairCounts {
			ga, gb := remap[p[0]], remap[p[1]]
			if ga > gb {
				ga, gb = gb, ga
			}
			a.pairCounts[ipair{ga, gb}] += count
		}
		for p, count := range lc.bigramCounts {
			ga, gb := remap[p[0]], remap[p[1]]
			a.bigramCounts[ipair{ga, gb}] += count
		}
		for p, count := range lc.skipGramCounts {
			ga, gb := remap[p[0]], remap[p[1]]
			if ga > gb {
				ga, gb = gb, ga
			}
			a.skipGramCounts[ipair{ga, gb}] += count
		}
	}
}

// Stats exposes the aggregated counts.
type Stats struct {
	TotalDocs      int64
	TokenDF        map[string]int64
	TokenCats      map[string]map[string]int64
	PairCounts     map[ipair]int64 // interned keys
	BigramCounts   map[ipair]int64
	SkipGramCounts map[ipair]int64
	internRev      []string // shared reference for ID→string lookup
	calc           *pmi.Calculator
	useNPMI        bool
	dampingCfg     *signals.DampingConfig
	dampingCache   map[string]float64
}

// tok returns the string for an interned token ID.
func (s *Stats) tok(id int32) string {
	return s.internRev[id]
}

// lookupID returns the interned ID for a token string, or -1 if not found.
func (s *Stats) lookupID(tok string) (int32, bool) {
	for i, t := range s.internRev {
		if t == tok {
			return int32(i), true
		}
	}
	return -1, false
}

// PairCount returns the document-level co-occurrence count for a sorted pair.
func (s *Stats) PairCount(a, b string) int64 {
	idA, okA := s.lookupID(a)
	idB, okB := s.lookupID(b)
	if !okA || !okB {
		return 0
	}
	if idA > idB {
		idA, idB = idB, idA
	}
	return s.PairCounts[ipair{idA, idB}]
}

// BigramCount returns the bigram count for an ordered pair.
func (s *Stats) BigramCount(a, b string) int64 {
	idA, okA := s.lookupID(a)
	idB, okB := s.lookupID(b)
	if !okA || !okB {
		return 0
	}
	return s.BigramCounts[ipair{idA, idB}]
}

// SkipGramCount returns the skip-gram count for a sorted pair.
func (s *Stats) SkipGramCount(a, b string) int64 {
	idA, okA := s.lookupID(a)
	idB, okB := s.lookupID(b)
	if !okA || !okB {
		return 0
	}
	if idA > idB {
		idA, idB = idB, idA
	}
	return s.SkipGramCounts[ipair{idA, idB}]
}

// SnapshotView returns statistics sharing the analyzer's maps (no copy).
func (a *Analyzer) SnapshotView() Stats {
	return Stats{
		TotalDocs:      a.totalDocs,
		TokenDF:        a.tokenDF,
		TokenCats:      a.tokenCats,
		PairCounts:     a.pairCounts,
		BigramCounts:   a.bigramCounts,
		SkipGramCounts: a.skipGramCounts,
		internRev:      a.internRev,
		calc:           a.calc,
		useNPMI:        a.pmiCfg.UseNPMI,
		dampingCfg:     a.dampingCfg,
	}
}

// Snapshot returns a copy of the accumulated statistics.
func (a *Analyzer) Snapshot() Stats {
	copyCats := make(map[string]map[string]int64, len(a.tokenCats))
	for tok, cats := range a.tokenCats {
		copyCats[tok] = make(map[string]int64, len(cats))
		for cat, count := range cats {
			copyCats[tok][cat] = count
		}
	}
	copyDF := make(map[string]int64, len(a.tokenDF))
	for tok, count := range a.tokenDF {
		copyDF[tok] = count
	}
	copyPairs := make(map[ipair]int64, len(a.pairCounts))
	for p, count := range a.pairCounts {
		copyPairs[p] = count
	}
	copyBigrams := make(map[ipair]int64, len(a.bigramCounts))
	for p, count := range a.bigramCounts {
		copyBigrams[p] = count
	}
	copySkipGrams := make(map[ipair]int64, len(a.skipGramCounts))
	for p, count := range a.skipGramCounts {
		copySkipGrams[p] = count
	}
	// internRev is append-only; share the slice (safe as long as analyzer isn't used after).
	return Stats{
		TotalDocs:      a.totalDocs,
		TokenDF:        copyDF,
		TokenCats:      copyCats,
		PairCounts:     copyPairs,
		BigramCounts:   copyBigrams,
		SkipGramCounts: copySkipGrams,
		internRev:      a.internRev,
		calc:           a.calc,
		useNPMI:        a.pmiCfg.UseNPMI,
		dampingCfg:     a.dampingCfg,
	}
}

// dampingFactor returns the damping factor for a token.
func (s *Stats) dampingFactor(token string) float64 {
	if s.dampingCfg == nil {
		return 1.0
	}
	if s.dampingCache == nil {
		s.buildDampingCache()
	}
	if f, ok := s.dampingCache[token]; ok {
		return f
	}
	return 1.0
}

// buildDampingCache computes damping factors for all tokens from pair statistics.
func (s *Stats) buildDampingCache() {
	s.dampingCache = make(map[string]float64, len(s.TokenDF))

	if s.TotalDocs == 0 || s.dampingCfg == nil {
		return
	}

	vocabSize := len(s.TokenDF)
	if vocabSize == 0 {
		return
	}

	neighborCount := make(map[string]int, len(s.TokenDF))
	for p, count := range s.PairCounts {
		if count == 0 {
			continue
		}
		tokA := s.tok(p[0])
		tokB := s.tok(p[1])
		dfA := s.TokenDF[tokA]
		dfB := s.TokenDF[tokB]
		if dfA == 0 || dfB == 0 {
			continue
		}
		score := s.computeScoreRaw(count, dfA, dfB, s.TotalDocs)
		if score >= s.dampingCfg.PMIThreshold {
			neighborCount[tokA]++
			neighborCount[tokB]++
		}
	}

	for tok := range s.TokenDF {
		density := signals.ComputeDamping(tok, &statsDensityProvider{
			count:     neighborCount[tok],
			vocabSize: vocabSize,
		}, *s.dampingCfg)
		s.dampingCache[tok] = density.DampingFactor
	}
}

// computeScoreRaw computes PMI/NPMI without damping.
func (s Stats) computeScoreRaw(pairCount, dfA, dfB, totalDocs int64) float64 {
	if dfA == 0 || dfB == 0 || totalDocs == 0 {
		return 0
	}
	c := s.calc
	if c == nil {
		c = pmi.NewCalculator(pmi.DefaultConfig().Epsilon)
	}
	return c.Score(pairCount, dfA, dfB, totalDocs, s.useNPMI)
}

type statsDensityProvider struct {
	count     int
	vocabSize int
}

func (p *statsDensityProvider) NeighborCount(_ string, _ float64) int { return p.count }
func (p *statsDensityProvider) VocabSize() int                       { return p.vocabSize }

// ComputedStats holds pre-computed results from a fused pair iteration.
type ComputedStats struct {
	StopwordStats []stoplist.Stats
	HighPMIPairs  []HighPMIPair
}

// ComputeAll computes stopword stats and high PMI pairs in a fused 2-pass operation.
// Pass 1: single map iteration — compute raw PMI, count neighbors for damping, cache pair data.
// Then compute damping factors from neighbor counts.
// Pass 2: iterate cached slice — apply damping, compute PMIMax + collect HighPMIPairs.
// This replaces 3 separate map iterations (buildDampingCache + StopwordStats + HighPMIPairs).
func (s *Stats) ComputeAll() ComputedStats {
	if s.TotalDocs == 0 {
		return ComputedStats{}
	}

	type pairInfo struct {
		tokA, tokB string
		rawPMI     float64
		count      int64
	}

	// Pass 1: single PairCounts iteration — raw PMI + neighbor counting.
	pairs := make([]pairInfo, 0, len(s.PairCounts))
	var neighborCount map[string]int
	if s.dampingCfg != nil {
		neighborCount = make(map[string]int, len(s.TokenDF))
	}

	for p, count := range s.PairCounts {
		if count == 0 {
			continue
		}
		tokA := s.tok(p[0])
		tokB := s.tok(p[1])
		dfA := s.TokenDF[tokA]
		dfB := s.TokenDF[tokB]
		if dfA == 0 || dfB == 0 {
			continue
		}
		rawScore := s.computeScoreRaw(count, dfA, dfB, s.TotalDocs)
		pairs = append(pairs, pairInfo{tokA: tokA, tokB: tokB, rawPMI: rawScore, count: count})

		if s.dampingCfg != nil && rawScore >= s.dampingCfg.PMIThreshold {
			neighborCount[tokA]++
			neighborCount[tokB]++
		}
	}

	// Compute damping factors from neighbor counts.
	dampingFactors := make(map[string]float64, len(s.TokenDF))
	if s.dampingCfg != nil {
		vocabSize := len(s.TokenDF)
		for tok := range s.TokenDF {
			density := signals.ComputeDamping(tok, &statsDensityProvider{
				count:     neighborCount[tok],
				vocabSize: vocabSize,
			}, *s.dampingCfg)
			dampingFactors[tok] = density.DampingFactor
		}
	}

	getDamping := func(tok string) float64 {
		if s.dampingCfg == nil {
			return 1.0
		}
		if f, ok := dampingFactors[tok]; ok {
			return f
		}
		return 1.0
	}

	// Pass 2: iterate cached slice (no map probing) — apply damping, compute PMIMax + HighPMIPairs.
	pmiMax := make(map[string]float64, len(s.TokenDF))
	highPairs := make([]HighPMIPair, 0, len(pairs))

	for i := range pairs {
		pi := &pairs[i]
		dampA := getDamping(pi.tokA)
		dampB := getDamping(pi.tokB)
		dampedPMI := pi.rawPMI * math.Sqrt(dampA*dampB)

		if dampedPMI > pmiMax[pi.tokA] {
			pmiMax[pi.tokA] = dampedPMI
		}
		if dampedPMI > pmiMax[pi.tokB] {
			pmiMax[pi.tokB] = dampedPMI
		}

		a, b := pi.tokA, pi.tokB
		if a > b {
			a, b = b, a
		}
		highPairs = append(highPairs, HighPMIPair{
			A:       a,
			B:       b,
			PMI:     dampedPMI,
			Support: pi.count,
		})
	}

	// Build stopword stats from PMIMax + TokenDF.
	stopStats := make([]stoplist.Stats, 0, len(s.TokenDF))
	for tok, df := range s.TokenDF {
		dfPercent := 100 * (float64(df) / float64(s.TotalDocs))
		ent := entropy(s.TokenCats[tok])
		stopStats = append(stopStats, stoplist.Stats{
			Token:      tok,
			DF:         df,
			DFPercent:  dfPercent,
			IDF:        math.Log(float64(s.TotalDocs) / (1 + float64(df))),
			PMIMax:     pmiMax[tok],
			CatEntropy: ent,
		})
	}

	return ComputedStats{
		StopwordStats: stopStats,
		HighPMIPairs:  highPairs,
	}
}

// StopwordStats converts corpus stats into the format expected by autotune/stopwords.
func (s *Stats) StopwordStats() []stoplist.Stats {
	return s.ComputeAll().StopwordStats
}

func entropy(counts map[string]int64) float64 {
	if len(counts) == 0 {
		return 0
	}
	var total float64
	for _, c := range counts {
		total += float64(c)
	}
	if total == 0 {
		return 0
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / total
		if p > 0 {
			h -= p * math.Log2(p)
		}
	}
	return h / math.Log2(float64(len(counts))+1)
}

// StopwordStatsProvider adapts Analyzer stats to the autotune interface.
type StopwordStatsProvider struct {
	stats *Stats
}

func NewStopwordStatsProvider(stats Stats) *StopwordStatsProvider {
	return &StopwordStatsProvider{stats: &stats}
}

func (p *StopwordStatsProvider) StopwordStats(ctx context.Context) ([]stoplist.Stats, error) {
	return p.stats.StopwordStats(), nil
}

// HighPMIPair describes a token pair with PMI and co-occurrence support.
type HighPMIPair struct {
	A       string
	B       string
	PMI     float64
	Support int64
}

// HighPMIPairs returns all document-level co-occurrence pairs with their PMI scores.
func (s *Stats) HighPMIPairs() []HighPMIPair {
	return s.ComputeAll().HighPMIPairs
}

// PairStat describes combined metrics for a token pair.
type PairStat struct {
	A           string
	B           string
	PMI         float64
	BigramFreq  int64
	Support     int64
	PhraseScore float64
}

// TopPairs returns phrase candidates ranked by combined bigram frequency + document PMI.
func (s *Stats) TopPairs(limit int, minPMI float64) []PairStat {
	if s.TotalDocs == 0 {
		return nil
	}
	var stats []PairStat

	for p, bigramCount := range s.BigramCounts {
		if bigramCount == 0 {
			continue
		}
		tokA := s.tok(p[0])
		tokB := s.tok(p[1])
		dfA := s.TokenDF[tokA]
		dfB := s.TokenDF[tokB]
		if dfA == 0 || dfB == 0 {
			continue
		}

		// PairCounts uses sorted pairs — normalize lookup
		a, b := p[0], p[1]
		if a > b {
			a, b = b, a
		}
		docPairCount := s.PairCounts[ipair{a, b}]
		if docPairCount == 0 {
			continue
		}
		rawScore := s.computeScoreRaw(docPairCount, dfA, dfB, s.TotalDocs)
		dampA := s.dampingFactor(tokA)
		dampB := s.dampingFactor(tokB)
		pmiVal := rawScore * math.Sqrt(dampA*dampB)
		if pmiVal < minPMI {
			continue
		}

		phraseScore := float64(bigramCount) * pmiVal

		stats = append(stats, PairStat{
			A:           tokA,
			B:           tokB,
			PMI:         pmiVal,
			BigramFreq:  bigramCount,
			Support:     docPairCount,
			PhraseScore: phraseScore,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].PhraseScore == stats[j].PhraseScore {
			return stats[i].BigramFreq > stats[j].BigramFreq
		}
		return stats[i].PhraseScore > stats[j].PhraseScore
	})

	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}
	return stats
}

// CTokenPair represents a c-token relationship with PMI and support.
type CTokenPair struct {
	TokenA  string
	TokenB  string
	PMI     float64
	Support int64
}

// CTokenPairs converts skip-gram statistics to c-token pairs with PMI scores.
func (s *Stats) CTokenPairs(minSupport int64) []CTokenPair {
	var pairs []CTokenPair

	for p, count := range s.SkipGramCounts {
		if count < minSupport {
			continue
		}

		tokA := s.tok(p[0])
		tokB := s.tok(p[1])
		dfA := s.TokenDF[tokA]
		dfB := s.TokenDF[tokB]
		if dfA == 0 || dfB == 0 {
			continue
		}

		rawScore := s.computeScoreRaw(count, dfA, dfB, s.TotalDocs)
		dampA := s.dampingFactor(tokA)
		dampB := s.dampingFactor(tokB)
		pmiVal := rawScore * math.Sqrt(dampA*dampB)

		pairs = append(pairs, CTokenPair{
			TokenA:  tokA,
			TokenB:  tokB,
			PMI:     pmiVal,
			Support: count,
		})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].PMI > pairs[j].PMI
	})

	return pairs
}

// RemoveTokens prunes tokens from accumulated statistics.
func (a *Analyzer) RemoveTokens(tokens []string) {
	if len(tokens) == 0 {
		return
	}
	stopIDs := make(map[int32]struct{}, len(tokens))
	for _, tok := range tokens {
		if id, ok := a.intern[tok]; ok {
			stopIDs[id] = struct{}{}
		}
		delete(a.tokenDF, tok)
		delete(a.tokenCats, tok)
	}
	for p := range a.pairCounts {
		if _, ok := stopIDs[p[0]]; ok {
			delete(a.pairCounts, p)
			continue
		}
		if _, ok := stopIDs[p[1]]; ok {
			delete(a.pairCounts, p)
		}
	}
	for p := range a.bigramCounts {
		if _, ok := stopIDs[p[0]]; ok {
			delete(a.bigramCounts, p)
			continue
		}
		if _, ok := stopIDs[p[1]]; ok {
			delete(a.bigramCounts, p)
		}
	}
	for p := range a.skipGramCounts {
		if _, ok := stopIDs[p[0]]; ok {
			delete(a.skipGramCounts, p)
			continue
		}
		if _, ok := stopIDs[p[1]]; ok {
			delete(a.skipGramCounts, p)
		}
	}
}
