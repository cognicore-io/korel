package analytics

import (
	"context"
	"math"
	"sort"

	"github.com/cognicore/korel/pkg/korel/stoplist"
)

// Skip-gram window configuration constants
const (
	// DefaultSkipGramWindow is the default distance for c-token tracking.
	// Tokens within this distance (default: 5 positions) are considered
	// contextually related for skip-gram co-occurrence statistics.
	DefaultSkipGramWindow = 5

	// MinSkipGramWindow is the minimum valid window size.
	// Values below this are clamped to ensure skip-gram tracking works.
	MinSkipGramWindow = 2
)

// Analyzer aggregates document-level token/category stats.
type Analyzer struct {
	totalDocs      int64
	tokenDF        map[string]int64
	tokenCats      map[string]map[string]int64
	pairCounts     map[pair]int64 // document-level co-occurrence
	bigramCounts   map[pair]int64 // adjacent token pairs only
	skipGramCounts map[pair]int64 // window-based co-occurrence (c-tokens)
	windowSize     int            // skip-gram window size (default: 5)
}

// NewAnalyzer creates an empty analyzer with default window size.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		tokenDF:        make(map[string]int64),
		tokenCats:      make(map[string]map[string]int64),
		pairCounts:     make(map[pair]int64),
		bigramCounts:   make(map[pair]int64),
		skipGramCounts: make(map[pair]int64),
		windowSize:     DefaultSkipGramWindow,
	}
}

// NewAnalyzerWithWindow creates an analyzer with a custom skip-gram window size.
// Window size must be >= MinSkipGramWindow; values below are clamped.
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

	seen := make(map[string]struct{})
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

	// Document-level pair counts (all unique tokens in document)
	unique := make([]string, 0, len(seen))
	for tok := range seen {
		unique = append(unique, tok)
	}
	sort.Strings(unique)
	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			a.pairCounts[newPair(unique[i], unique[j])]++
		}
	}

	// Bigram counts (adjacent tokens only, preserving order)
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i] == "" || tokens[i+1] == "" {
			continue
		}
		// Use ordered pair for bigrams (don't sort alphabetically)
		p := pair{A: tokens[i], B: tokens[i+1]}
		a.bigramCounts[p]++
	}

	// Skip-gram counts (window-based co-occurrence for c-tokens)
	// Tracks tokens that appear within a configurable window (default: 5)
	// Used for contextual token (c-token) relationships
	// Deduplicated per document: each unique pair counted once even if it appears in multiple windows
	skipGramSeen := make(map[pair]struct{})
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "" {
			continue
		}
		// Look ahead within window
		for j := i + 1; j < len(tokens) && j < i+a.windowSize; j++ {
			if tokens[j] == "" || tokens[j] == tokens[i] {
				continue // Skip empty and self-pairs
			}
			// Use alphabetically sorted pair (like pairCounts)
			p := newPair(tokens[i], tokens[j])
			if _, seen := skipGramSeen[p]; !seen {
				skipGramSeen[p] = struct{}{}
			}
		}
	}
	// Increment counts for all unique skip-gram pairs found in this document
	for p := range skipGramSeen {
		a.skipGramCounts[p]++
	}
}

// Stats exposes the aggregated counts.
type Stats struct {
	TotalDocs      int64
	TokenDF        map[string]int64
	TokenCats      map[string]map[string]int64
	PairCounts     map[pair]int64 // document-level co-occurrence
	BigramCounts   map[pair]int64 // adjacent token pairs
	SkipGramCounts map[pair]int64 // window-based co-occurrence (c-tokens)
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
	copyPairs := make(map[pair]int64, len(a.pairCounts))
	for p, count := range a.pairCounts {
		copyPairs[p] = count
	}
	copyBigrams := make(map[pair]int64, len(a.bigramCounts))
	for p, count := range a.bigramCounts {
		copyBigrams[p] = count
	}
	copySkipGrams := make(map[pair]int64, len(a.skipGramCounts))
	for p, count := range a.skipGramCounts {
		copySkipGrams[p] = count
	}
	return Stats{
		TotalDocs:      a.totalDocs,
		TokenDF:        copyDF,
		TokenCats:      copyCats,
		PairCounts:     copyPairs,
		BigramCounts:   copyBigrams,
		SkipGramCounts: copySkipGrams,
	}
}

// StopwordStats converts corpus stats into the format expected by autotune/stopwords.
// Computes PMIMax for each token: the maximum PMI across all pairs containing that token.
// Stopword candidates have high DF but low PMIMax (appear everywhere but don't associate strongly).
func (s Stats) StopwordStats() []stoplist.Stats {
	var out []stoplist.Stats
	if s.TotalDocs == 0 {
		return out
	}

	// Calculate PMIMax for each token by finding the max PMI among all pairs containing it
	pmiMax := make(map[string]float64)
	for p, count := range s.PairCounts {
		if count == 0 {
			continue
		}
		dfA := s.TokenDF[p.A]
		dfB := s.TokenDF[p.B]
		if dfA == 0 || dfB == 0 {
			continue
		}
		pmi := computePMI(count, dfA, dfB, s.TotalDocs)

		// Track max PMI for both tokens in the pair
		if pmi > pmiMax[p.A] {
			pmiMax[p.A] = pmi
		}
		if pmi > pmiMax[p.B] {
			pmiMax[p.B] = pmi
		}
	}

	// Build stopword stats with computed PMIMax
	for tok, df := range s.TokenDF {
		dfPercent := 100 * (float64(df) / float64(s.TotalDocs))
		entropy := entropy(s.TokenCats[tok])
		out = append(out, stoplist.Stats{
			Token:      tok,
			DF:         df,
			DFPercent:  dfPercent,
			IDF:        math.Log(float64(s.TotalDocs) / (1 + float64(df))),
			PMIMax:     pmiMax[tok], // Now computed from actual pair statistics
			CatEntropy: entropy,
		})
	}
	return out
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
	stats Stats
}

func NewStopwordStatsProvider(stats Stats) *StopwordStatsProvider {
	return &StopwordStatsProvider{stats: stats}
}

func (p *StopwordStatsProvider) StopwordStats(ctx context.Context) ([]stoplist.Stats, error) {
	return p.stats.StopwordStats(), nil
}

// PairStat describes combined metrics for a token pair.
type PairStat struct {
	A             string
	B             string
	PMI           float64 // document-level semantic relatedness
	BigramFreq    int64   // adjacency count (how often they appear next to each other)
	Support       int64   // document co-occurrence count (deprecated, same as doc-level)
	PhraseScore   float64 // combined score: bigramFreq * PMI (higher = better phrase)
}

// TopPairs returns phrase candidates ranked by combined bigram frequency + document PMI.
// For phrase discovery, this filters out:
// - Stopword adjacencies (high bigram, low PMI): "the model", "a new"
// - Non-collocations (low bigram, high PMI): "learning theory" (conceptually related but not a fixed phrase)
func (s Stats) TopPairs(limit int, minPMI float64) []PairStat {
	if s.TotalDocs == 0 {
		return nil
	}
	var stats []PairStat

	// Iterate over bigrams (adjacent pairs) as candidates
	for p, bigramCount := range s.BigramCounts {
		if bigramCount == 0 {
			continue
		}
		dfA := s.TokenDF[p.A]
		dfB := s.TokenDF[p.B]
		if dfA == 0 || dfB == 0 {
			continue
		}

		// Calculate document-level PMI for semantic filtering
		// PairCounts uses sorted pairs, so normalize the lookup
		docPairCount := s.PairCounts[newPair(p.A, p.B)]
		if docPairCount == 0 {
			continue // shouldn't happen, but safety check
		}
		pmi := computePMI(docPairCount, dfA, dfB, s.TotalDocs)
		if pmi < minPMI {
			continue // filter out weak semantic associations
		}

		// Combined score: adjacency frequency weighted by semantic strength
		phraseScore := float64(bigramCount) * pmi

		stats = append(stats, PairStat{
			A:            p.A,
			B:            p.B,
			PMI:          pmi,
			BigramFreq:   bigramCount,
			Support:      docPairCount,
			PhraseScore:  phraseScore,
		})
	}

	// Sort by phrase score (bigram freq * PMI), then by bigram freq
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

func computePMI(pairCount, dfA, dfB, totalDocs int64) float64 {
	if dfA == 0 || dfB == 0 || totalDocs == 0 {
		return 0
	}
	smooth := 1.0
	numerator := (float64(pairCount) + smooth) / float64(totalDocs)
	denominator := ((float64(dfA) + smooth) / float64(totalDocs)) * ((float64(dfB) + smooth) / float64(totalDocs))
	return math.Log(numerator / denominator)
}

// CTokenPair represents a c-token relationship with PMI and support.
type CTokenPair struct {
	TokenA  string
	TokenB  string
	PMI     float64
	Support int64
}

// CTokenPairs converts skip-gram statistics to c-token pairs with PMI scores.
// Only returns pairs with support >= minSupport.
// This provides a clean API for building lexicon c-token relationships from corpus statistics.
func (s Stats) CTokenPairs(minSupport int64) []CTokenPair {
	var pairs []CTokenPair

	for p, count := range s.SkipGramCounts {
		if count < minSupport {
			continue
		}

		dfA := s.TokenDF[p.A]
		dfB := s.TokenDF[p.B]
		if dfA == 0 || dfB == 0 {
			continue
		}

		pmi := computePMI(count, dfA, dfB, s.TotalDocs)

		pairs = append(pairs, CTokenPair{
			TokenA:  p.A,
			TokenB:  p.B,
			PMI:     pmi,
			Support: count,
		})
	}

	// Sort by PMI (descending) for convenience
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].PMI > pairs[j].PMI
	})

	return pairs
}

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
