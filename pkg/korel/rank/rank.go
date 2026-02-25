package rank

import (
	"math"
	"time"
)

// Scorer calculates hybrid scores for document ranking
type Scorer struct {
	weights      Weights
	halfLifeDays float64
}

// Weights defines the scoring weights
type Weights struct {
	AlphaPMI     float64 // PMI importance
	BetaCats     float64 // category overlap
	GammaRecency float64 // time decay
	EtaAuthority float64 // link authority
	DeltaLen     float64 // length penalty
}

// NewScorer creates a new scorer with the given weights
func NewScorer(w Weights, halfLifeDays float64) *Scorer {
	return &Scorer{
		weights:      w,
		halfLifeDays: halfLifeDays,
	}
}

// Candidate represents a document to be scored
type Candidate struct {
	DocID       int64
	Tokens      []string
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
	Cats      float64
	Recency   float64
	Authority float64
	Len       float64
	Damping   float64 // average damping factor applied to PMI (1.0 = no damping)
	Total     float64
}

// ScoreWithBreakdown calculates score with detailed breakdown.
// An optional dampingMap scales the PMI contribution per query token.
// Hub tokens (connecting to many neighbors) should have damping < 1.0.
// Pass nil for no damping.
func (s *Scorer) ScoreWithBreakdown(query Query, candidate Candidate, now time.Time, pmiFunc func(qt, dt string) float64, dampingMap ...map[string]float64) ScoreBreakdown {
	var dmap map[string]float64
	if len(dampingMap) > 0 {
		dmap = dampingMap[0]
	}

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

	catOverlap := jaccard(query.Categories, candidate.Categories)
	ageDays := now.Sub(candidate.PublishedAt).Hours() / 24.0
	recency := math.Exp(-ageDays / s.halfLifeDays)
	authority := math.Log(float64(candidate.LinksOut + 1))
	lenPenalty := math.Log(float64(len(candidate.Tokens) + 1))

	breakdown := ScoreBreakdown{
		PMI:       s.weights.AlphaPMI * pmiPart,
		Cats:      s.weights.BetaCats * catOverlap,
		Recency:   s.weights.GammaRecency * recency,
		Authority: s.weights.EtaAuthority * authority,
		Len:       s.weights.DeltaLen * lenPenalty,
		Damping:   avgDamping,
	}
	breakdown.Total = breakdown.PMI + breakdown.Cats + breakdown.Recency + breakdown.Authority - breakdown.Len

	return breakdown
}

// Query represents a parsed user query
type Query struct {
	Tokens     []string
	Categories []string
}

// jaccard calculates Jaccard similarity between two string slices
func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
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
