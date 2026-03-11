// Package reltype provides distributional relationship classification
// for typed query expansion. It infers relationship types (synonym,
// broader, narrower, related) from corpus statistics that Korel already
// computes (co-occurrence counts, document frequencies, PMI).
//
// The classifier is optional — when disabled, all PMI edges remain
// untyped "related_to" as before. When enabled, BuildGraph classifies
// edges into typed relations that power directional query expansion.
//
// Theory:
//   - Synonym detection uses the "same neighbors, low co-occurrence"
//     signal: true synonyms share distributional contexts but rarely
//     appear together in the same document.
//   - Hypernymy (broader/narrower) uses the distributional inclusion
//     hypothesis (Weeds & Weir 2003, Kotlerman et al. 2010): if A's
//     co-occurrence contexts are a subset of B's, then A is narrower
//     than B.
//   - Everything else remains "related_to" (symmetric association).
package reltype

import (
	"math"
	"sort"
)

// RelType represents the inferred relationship type between two tokens.
type RelType string

const (
	Related  RelType = "related_to" // symmetric, default
	SameAs   RelType = "same_as"    // synonym / alias
	Broader  RelType = "broader"    // hypernym (B is more general than A)
	Narrower RelType = "narrower"   // hyponym (B is more specific than A)
)

// Config controls the thresholds for relationship classification.
type Config struct {
	// MinNeighborOverlap is the minimum Jaccard similarity of top-K
	// neighbor sets for two tokens to be considered synonym candidates.
	// Higher = stricter synonym detection. Default: 0.3
	MinNeighborOverlap float64

	// MaxCooccurrenceRatio is the maximum P(A,B)/min(P(A),P(B)) for
	// synonym candidates. True synonyms rarely co-occur in the same
	// document because writers pick one form or the other. Default: 0.5
	MaxCooccurrenceRatio float64

	// InclusionThreshold is the minimum Weeds precision (directional
	// inclusion) score to classify a pair as broader/narrower. Default: 0.7
	InclusionThreshold float64

	// MinConfidence is the minimum confidence score for any typed
	// classification. Below this, the edge stays as "related_to". Default: 0.5
	MinConfidence float64

	// TopK is the number of top neighbors to compare for overlap
	// and inclusion calculations. Default: 20
	TopK int
}

// DefaultConfig returns production-ready thresholds.
func DefaultConfig() Config {
	return Config{
		MinNeighborOverlap:   0.3,
		MaxCooccurrenceRatio: 0.5,
		InclusionThreshold:   0.7,
		MinConfidence:        0.5,
		TopK:                 20,
	}
}

// Neighbor is a token with its PMI score (input to the classifier).
type Neighbor struct {
	Token string
	PMI   float64
}

// Classification is the result of classifying a token pair.
type Classification struct {
	Type       RelType
	Confidence float64 // 0.0–1.0
}

// Classifier infers relationship types from distributional statistics.
type Classifier struct {
	cfg Config
}

// NewClassifier creates a relationship classifier with the given config.
func NewClassifier(cfg Config) *Classifier {
	if cfg.TopK <= 0 {
		cfg.TopK = DefaultConfig().TopK
	}
	return &Classifier{cfg: cfg}
}

// TopK returns the configured number of neighbors to compare.
func (c *Classifier) TopK() int { return c.cfg.TopK }

// Classify determines the relationship type between tokens A and B
// given their neighbor sets, document frequencies, and co-occurrence count.
//
// Parameters:
//   - neighborsA, neighborsB: top-K PMI neighbors for each token
//   - dfA, dfB: document frequency of each token
//   - coAB: number of documents containing both A and B
//   - totalDocs: corpus size
func (c *Classifier) Classify(
	neighborsA, neighborsB []Neighbor,
	dfA, dfB, coAB, totalDocs int64,
) Classification {
	// 1. Check synonym signal: high neighbor overlap + low co-occurrence
	if synConf := c.synonymScore(neighborsA, neighborsB, dfA, dfB, coAB); synConf >= c.cfg.MinConfidence {
		return Classification{Type: SameAs, Confidence: synConf}
	}

	// 2. Check hypernymy via distributional inclusion
	weedsAB := c.weedsPrecision(neighborsA, neighborsB)
	weedsBA := c.weedsPrecision(neighborsB, neighborsA)

	if weedsAB >= c.cfg.InclusionThreshold && weedsAB > weedsBA+0.1 {
		// A's contexts ⊂ B's contexts → B is broader than A
		conf := math.Min(weedsAB, 1.0)
		if conf >= c.cfg.MinConfidence {
			return Classification{Type: Broader, Confidence: conf}
		}
	}
	if weedsBA >= c.cfg.InclusionThreshold && weedsBA > weedsAB+0.1 {
		// B's contexts ⊂ A's contexts → B is narrower than A
		conf := math.Min(weedsBA, 1.0)
		if conf >= c.cfg.MinConfidence {
			return Classification{Type: Narrower, Confidence: conf}
		}
	}

	// 3. Default: symmetric relatedness
	return Classification{Type: Related, Confidence: 1.0}
}

// synonymScore computes a confidence that A and B are synonyms.
// Uses two signals:
//   - High Jaccard overlap of top-K neighbor sets (they live in the same contexts)
//   - Low co-occurrence ratio (writers use one or the other, not both)
func (c *Classifier) synonymScore(
	neighborsA, neighborsB []Neighbor,
	dfA, dfB, coAB int64,
) float64 {
	overlap := neighborJaccard(neighborsA, neighborsB, c.cfg.TopK)
	if overlap < c.cfg.MinNeighborOverlap {
		return 0
	}

	// Co-occurrence ratio: how often do they appear together vs. the rarer term?
	minDF := dfA
	if dfB < minDF {
		minDF = dfB
	}
	if minDF == 0 {
		return 0
	}
	coRatio := float64(coAB) / float64(minDF)
	if coRatio > c.cfg.MaxCooccurrenceRatio {
		return 0 // they co-occur too much to be synonyms
	}

	// Confidence: neighbor overlap is the primary signal, penalized by co-occurrence
	conf := overlap * (1.0 - coRatio)
	return conf
}

// weedsPrecision computes the Weeds precision (directional inclusion score).
// WP(A→B) = |neighbors(A) ∩ neighbors(B)| / |neighbors(A)|
// High WP(A→B) means A's contexts are included in B's contexts,
// suggesting B is more general (broader) than A.
func (c *Classifier) weedsPrecision(neighborsA, neighborsB []Neighbor) float64 {
	if len(neighborsA) == 0 {
		return 0
	}

	setB := make(map[string]struct{}, len(neighborsB))
	limit := c.cfg.TopK
	for i, nb := range neighborsB {
		if i >= limit {
			break
		}
		setB[nb.Token] = struct{}{}
	}

	var overlap int
	count := 0
	for _, nb := range neighborsA {
		if count >= limit {
			break
		}
		count++
		if _, ok := setB[nb.Token]; ok {
			overlap++
		}
	}

	if count == 0 {
		return 0
	}
	return float64(overlap) / float64(count)
}

// neighborJaccard computes the Jaccard similarity of two neighbor sets,
// considering only the top K entries from each.
func neighborJaccard(a, b []Neighbor, topK int) float64 {
	setA := topKSet(a, topK)
	setB := topKSet(b, topK)

	if len(setA) == 0 && len(setB) == 0 {
		return 0
	}

	var intersection int
	for tok := range setA {
		if _, ok := setB[tok]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func topKSet(neighbors []Neighbor, k int) map[string]struct{} {
	// Neighbors should already be sorted by PMI desc, but be safe
	sorted := make([]Neighbor, len(neighbors))
	copy(sorted, neighbors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PMI > sorted[j].PMI
	})

	set := make(map[string]struct{}, k)
	for i, nb := range sorted {
		if i >= k {
			break
		}
		set[nb.Token] = struct{}{}
	}
	return set
}
