package korel

import (
	"math"
	"time"
)

// EvidenceScore rates the quality of evidence backing a single card.
type EvidenceScore struct {
	CardID        string
	Freshness     float64 // 0-1: how recent the sources are
	Corroboration float64 // 0-1: how many independent sources agree on tokens
	Authority     float64 // 0-1: normalized link authority
	Overall       float64 // weighted combination
}

// scoreEvidence computes evidence quality for a set of scored documents.
func scoreEvidence(docs []scored, now time.Time, halfLifeDays float64) EvidenceScore {
	if len(docs) == 0 {
		return EvidenceScore{}
	}

	// Freshness: average recency across sources (exponential decay)
	freshSum := 0.0
	for _, d := range docs {
		ageDays := math.Max(0, now.Sub(d.doc.PublishedAt).Hours()/24.0)
		freshSum += math.Exp(-ageDays / math.Max(1, halfLifeDays))
	}
	freshness := freshSum / float64(len(docs))

	// Corroboration: Jaccard similarity of token sets across documents.
	// More overlapping tokens across independent docs = stronger evidence.
	corroboration := 0.0
	if len(docs) >= 2 {
		totalPairs := 0
		overlapSum := 0.0
		for i := 0; i < len(docs); i++ {
			setA := toSet(docs[i].doc.Tokens)
			for j := i + 1; j < len(docs); j++ {
				setB := toSet(docs[j].doc.Tokens)
				overlapSum += jaccardSets(setA, setB)
				totalPairs++
			}
		}
		if totalPairs > 0 {
			corroboration = overlapSum / float64(totalPairs)
		}
	}

	// Authority: normalized log of link count
	authSum := 0.0
	for _, d := range docs {
		authSum += math.Log(float64(d.doc.LinksOut + 1))
	}
	avgAuth := authSum / float64(len(docs))
	// Normalize to 0-1 range (log(101) ≈ 4.6 is a reasonable cap)
	authority := math.Min(1.0, avgAuth/4.6)

	overall := 0.4*freshness + 0.35*corroboration + 0.25*authority

	return EvidenceScore{
		Freshness:     freshness,
		Corroboration: corroboration,
		Authority:     authority,
		Overall:       overall,
	}
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func jaccardSets(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
