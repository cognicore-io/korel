package analytics

import "sort"

// HighDFEntry is a token with its document-frequency percentage and category entropy.
type HighDFEntry struct {
	Token     string  `json:"token"`
	DFPercent float64 `json:"df_percent"`
	Entropy   float64 `json:"entropy"`
}

// TopHighDF returns the highest-DF tokens from corpus stats, sorted descending by DF%.
func TopHighDF(stats Stats, limit int) []HighDFEntry {
	entries := stats.StopwordStats()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DFPercent > entries[j].DFPercent
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]HighDFEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, HighDFEntry{
			Token:     e.Token,
			DFPercent: e.DFPercent,
			Entropy:   e.CatEntropy,
		})
	}
	return out
}
