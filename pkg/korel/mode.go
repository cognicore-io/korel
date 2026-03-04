package korel

import "strings"

// SearchMode represents the detected query intent.
type SearchMode string

const (
	ModeAuto    SearchMode = ""        // auto-detect from query
	ModeFact    SearchMode = "fact"    // direct factual answer
	ModeTrend   SearchMode = "trend"   // temporal pattern / change over time
	ModeCompare SearchMode = "compare" // side-by-side comparison
	ModeExplore SearchMode = "explore" // broad discovery
)

// WeightMultipliers are per-mode adjustments applied to the base scoring weights.
// A value of 1.0 means no change; >1 amplifies, <1 dampens.
type WeightMultipliers struct {
	PMI       float64
	Cats      float64
	Recency   float64
	Authority float64
	Len       float64
}

// DetectMode classifies a query string into a search mode using keyword heuristics.
func DetectMode(query string) SearchMode {
	lower := strings.ToLower(query)

	// Compare signals
	for _, kw := range []string{" vs ", " versus ", "compare ", "difference between "} {
		if strings.Contains(lower, kw) {
			return ModeCompare
		}
	}

	// Trend signals
	for _, kw := range []string{"trend", "over time", "change in ", "growth of ", "history of "} {
		if strings.Contains(lower, kw) {
			return ModeTrend
		}
	}

	// Fact signals — question words at the start
	for _, prefix := range []string{"what ", "who ", "when ", "where ", "how ", "why ", "is ", "does ", "define "} {
		if strings.HasPrefix(lower, prefix) {
			return ModeFact
		}
	}

	return ModeExplore
}

// ModeWeights returns scoring weight multipliers for the given search mode.
func ModeWeights(mode SearchMode) WeightMultipliers {
	switch mode {
	case ModeFact:
		// Fact queries: boost PMI (precision), reduce recency (timeless facts)
		return WeightMultipliers{PMI: 1.3, Cats: 1.0, Recency: 0.5, Authority: 1.2, Len: 1.0}
	case ModeTrend:
		// Trend queries: boost recency, moderate PMI
		return WeightMultipliers{PMI: 0.8, Cats: 1.0, Recency: 1.5, Authority: 1.0, Len: 0.8}
	case ModeCompare:
		// Comparison: boost category overlap (need matching topics), balanced PMI
		return WeightMultipliers{PMI: 1.0, Cats: 1.5, Recency: 0.8, Authority: 1.0, Len: 1.0}
	case ModeExplore:
		// Exploration: reduce PMI strictness, boost authority (find hubs)
		return WeightMultipliers{PMI: 0.7, Cats: 0.8, Recency: 1.0, Authority: 1.3, Len: 0.7}
	default:
		return WeightMultipliers{PMI: 1.0, Cats: 1.0, Recency: 1.0, Authority: 1.0, Len: 1.0}
	}
}

// OutputFormat controls how search results are rendered.
type OutputFormat string

const (
	FormatCards OutputFormat = ""          // default: structured cards
	FormatBrief OutputFormat = "briefing"  // executive briefing
	FormatMemo  OutputFormat = "memo"      // detailed memo with sources
	FormatDigest OutputFormat = "digest"   // compact bullet digest
	FormatWatch OutputFormat = "watchlist" // monitoring watchlist
)
