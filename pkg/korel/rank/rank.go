package rank

import (
	"math"
	"time"
)

// Scorer calculates hybrid scores for document ranking
type Scorer struct {
	weights      Weights
	halfLifeDays float64
	corpus       CorpusStats
	bm25         BM25Params
}

// Weights defines the scoring weights
type Weights struct {
	AlphaPMI     float64 // PMI importance
	BetaCats     float64 // category overlap
	GammaRecency float64 // time decay
	EtaAuthority float64 // link authority
	DeltaLen     float64 // length penalty
	ZetaBM25     float64 // BM25 term relevance
	IotaTitle    float64 // title match boost
}

// CorpusStats holds precomputed corpus-level statistics needed for BM25.
type CorpusStats struct {
	TotalDocs int64   // N — total documents in corpus
	AvgDocLen float64 // avgdl — average document length (in tokens)
}

// BM25Params holds BM25 tuning parameters.
type BM25Params struct {
	K1 float64 // term frequency saturation (default 1.2)
	B  float64 // length normalization (default 0.75)
}

// DefaultBM25Params returns standard BM25 parameters.
func DefaultBM25Params() BM25Params {
	return BM25Params{K1: 1.2, B: 0.35}
}

// NewScorer creates a new scorer with the given weights.
// Corpus stats and BM25 params default to zero/standard if not set via options.
func NewScorer(w Weights, halfLifeDays float64, opts ...ScorerOption) *Scorer {
	s := &Scorer{
		weights:      w,
		halfLifeDays: halfLifeDays,
		bm25:         DefaultBM25Params(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ScorerOption configures optional scorer parameters.
type ScorerOption func(*Scorer)

// WithCorpusStats sets corpus-level statistics for BM25 scoring.
func WithCorpusStats(cs CorpusStats) ScorerOption {
	return func(s *Scorer) { s.corpus = cs }
}

// WithBM25Params overrides the default BM25 parameters.
func WithBM25Params(p BM25Params) ScorerOption {
	return func(s *Scorer) { s.bm25 = p }
}

// Candidate represents a document to be scored
type Candidate struct {
	DocID       int64
	Tokens      []string
	TitleTokens []string // tokenized title for title-match boost
	Categories  []string
	PublishedAt time.Time
	LinksOut    int
}

// Score calculates a hybrid score for a candidate
//
// score = α·PMI + β·cat_overlap + γ·recency + η·authority - δ·len_penalty
func (s *Scorer) Score(query Query, candidate Candidate, now time.Time, pmiFunc func(qt, dt string) float64) float64 {
	return s.ScoreWithBreakdown(query, candidate, now, pmiFunc).Total
}

// ScoreBreakdown provides detailed scoring information
type ScoreBreakdown struct {
	PMI       float64
	BM25      float64 // BM25 term-relevance score
	Title     float64 // title match boost
	Cats      float64
	Recency   float64
	Authority float64
	Len       float64
	Damping   float64 // average damping factor applied to PMI (1.0 = no damping)
	Total     float64
}

// ScoreOpts bundles optional scoring parameters.
type ScoreOpts struct {
	DampingMap map[string]float64            // per-token PMI damping (nil = no damping)
	DFFunc     func(token string) int64      // document frequency lookup for BM25 IDF
}

// ScoreWithBreakdown calculates score with detailed breakdown.
// An optional dampingMap scales the PMI contribution per query token.
// Hub tokens (connecting to many neighbors) should have damping < 1.0.
// Pass nil for no damping.
func (s *Scorer) ScoreWithBreakdown(query Query, candidate Candidate, now time.Time, pmiFunc func(qt, dt string) float64, dampingMap ...map[string]float64) ScoreBreakdown {
	var opts ScoreOpts
	if len(dampingMap) > 0 {
		opts.DampingMap = dampingMap[0]
	}
	return s.ScoreWithOpts(query, candidate, now, pmiFunc, opts)
}

// ScoreWithOpts is the full scoring function with all optional parameters.
func (s *Scorer) ScoreWithOpts(query Query, candidate Candidate, now time.Time, pmiFunc func(qt, dt string) float64, opts ScoreOpts) ScoreBreakdown {
	dmap := opts.DampingMap

	// --- PMI component (unchanged) ---
	pmiSum := 0.0
	dampSum := 0.0
	for _, qt := range query.Tokens {
		maxPMI := 0.0
		for _, dt := range candidate.Tokens {
			pmi := pmiFunc(qt, dt)
			if pmi > maxPMI {
				maxPMI = pmi
			}
		}
		d := 1.0
		if dmap != nil {
			if v, ok := dmap[qt]; ok {
				d = v
			}
		}
		pmiSum += maxPMI * d
		dampSum += d
	}
	pmiPart := pmiSum / math.Max(1, float64(len(query.Tokens)))
	avgDamping := 1.0
	if len(query.Tokens) > 0 {
		avgDamping = dampSum / float64(len(query.Tokens))
	}

	// --- BM25 component ---
	bm25Score := 0.0
	if s.weights.ZetaBM25 > 0 && s.corpus.TotalDocs > 0 && opts.DFFunc != nil {
		bm25Score = s.computeBM25(query.Tokens, candidate.Tokens, opts.DFFunc)
	}

	// --- Title match component ---
	titleScore := 0.0
	if s.weights.IotaTitle > 0 && len(candidate.TitleTokens) > 0 {
		titleScore = s.computeTitleMatch(query.Tokens, candidate.TitleTokens)
	}

	catOverlap := jaccard(query.Categories, candidate.Categories)
	ageDays := math.Max(0, now.Sub(candidate.PublishedAt).Hours()/24.0)
	recency := math.Exp(-ageDays / s.halfLifeDays)
	authority := math.Log(float64(candidate.LinksOut + 1))
	lenPenalty := math.Log(float64(len(candidate.Tokens) + 1))

	breakdown := ScoreBreakdown{
		PMI:       s.weights.AlphaPMI * pmiPart,
		BM25:      s.weights.ZetaBM25 * bm25Score,
		Title:     s.weights.IotaTitle * titleScore,
		Cats:      s.weights.BetaCats * catOverlap,
		Recency:   s.weights.GammaRecency * recency,
		Authority: s.weights.EtaAuthority * authority,
		Len:       s.weights.DeltaLen * lenPenalty,
		Damping:   avgDamping,
	}
	breakdown.Total = breakdown.PMI + breakdown.BM25 + breakdown.Title + breakdown.Cats + breakdown.Recency + breakdown.Authority - breakdown.Len

	return breakdown
}

// computeBM25 calculates BM25 score for a document against query tokens.
// Uses Okapi BM25: sum over query terms of IDF(q) * (tf * (k1+1)) / (tf + k1 * (1 - b + b * dl/avgdl))
func (s *Scorer) computeBM25(queryTokens, docTokens []string, dfFunc func(string) int64) float64 {
	if len(queryTokens) == 0 || len(docTokens) == 0 {
		return 0
	}

	// Count term frequencies in document
	tf := make(map[string]int, len(docTokens))
	for _, t := range docTokens {
		tf[t]++
	}

	dl := float64(len(docTokens))
	avgdl := s.corpus.AvgDocLen
	if avgdl <= 0 {
		avgdl = dl // fallback
	}
	N := float64(s.corpus.TotalDocs)
	k1 := s.bm25.K1
	b := s.bm25.B

	score := 0.0
	for _, qt := range queryTokens {
		termFreq := float64(tf[qt])
		if termFreq == 0 {
			continue
		}

		// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
		df := float64(dfFunc(qt))
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)

		// BM25 term score
		numerator := termFreq * (k1 + 1)
		denominator := termFreq + k1*(1-b+b*dl/avgdl)
		score += idf * numerator / denominator
	}

	return score
}

// computeTitleMatch scores how well query tokens match the document title.
// Returns fraction of query tokens found in title, with a bonus for full coverage.
func (s *Scorer) computeTitleMatch(queryTokens, titleTokens []string) float64 {
	if len(queryTokens) == 0 || len(titleTokens) == 0 {
		return 0
	}

	titleSet := make(map[string]struct{}, len(titleTokens))
	for _, t := range titleTokens {
		titleSet[t] = struct{}{}
	}

	matched := 0
	for _, qt := range queryTokens {
		if _, ok := titleSet[qt]; ok {
			matched++
		}
	}

	coverage := float64(matched) / float64(len(queryTokens))

	// Bonus: if all query tokens appear in title, add extra boost
	if matched == len(queryTokens) {
		coverage = 1.5
	}

	return coverage
}

// Query represents a parsed user query
type Query struct {
	Tokens     []string
	Categories []string
}

// jaccard calculates Jaccard similarity between two string slices.
// Returns 0 when both slices are empty (no category info = no signal).
func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}

	aSet := make(map[string]struct{}, len(a))
	for _, s := range a {
		aSet[s] = struct{}{}
	}

	bSet := make(map[string]struct{}, len(b))
	for _, s := range b {
		bSet[s] = struct{}{}
	}

	intersection := 0
	for s := range aSet {
		if _, ok := bSet[s]; ok {
			intersection++
		}
	}

	union := len(aSet) + len(bSet) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}
