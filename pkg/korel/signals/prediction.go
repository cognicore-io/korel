package signals

// PredictionError compares what PMI statistics "predicted" should be relevant
// against what the retrieval actually returned.
//
// Inspired by Daimon's iPC (predictive coding): large prediction error = surprising
// = relevant. In Korel, this is a retrieval quality signal. High error means either
// the query found something genuinely new (discovery) or the retrieval is noisy.
type PredictionError struct {
	// Score is the Jaccard distance between predicted and actual token sets.
	// Range 0..1. Higher = more surprising results.
	Score float64

	// Predicted are the tokens PMI expected to be relevant (top neighbors).
	Predicted []string

	// Actual are the tokens found in the retrieval results.
	Actual []string

	// OnlyPredicted are tokens PMI expected but retrieval didn't find.
	// These might indicate gaps in the corpus or overly narrow retrieval.
	OnlyPredicted []string

	// OnlyActual are tokens found in results but not predicted by PMI.
	// These are potential discoveries — new connections the statistics
	// haven't captured yet.
	OnlyActual []string

	// Overlap are tokens that were both predicted and found.
	Overlap []string
}

// NeighborProvider returns PMI neighbors for a token.
type NeighborProvider interface {
	// TopNeighbors returns the top-k tokens most associated with the given token.
	TopNeighbors(token string, k int) []string
}

// PredictionConfig controls prediction error computation.
type PredictionConfig struct {
	// NeighborsPerToken is how many PMI neighbors to consider per query token.
	// Default: 10
	NeighborsPerToken int
}

// DefaultPredictionConfig returns sensible defaults.
func DefaultPredictionConfig() PredictionConfig {
	return PredictionConfig{
		NeighborsPerToken: 10,
	}
}

// ComputePredictionError measures the divergence between PMI-predicted tokens
// and the tokens actually found in retrieval results.
//
// queryTokens: the tokens from the user's query
// resultTokens: unique tokens extracted from all retrieved documents
// provider: returns PMI neighbors for each query token
func ComputePredictionError(queryTokens, resultTokens []string, provider NeighborProvider, cfg PredictionConfig) PredictionError {
	if provider == nil || len(queryTokens) == 0 {
		return PredictionError{}
	}

	if cfg.NeighborsPerToken <= 0 {
		cfg.NeighborsPerToken = 10
	}

	// Build predicted set: union of top neighbors for each query token
	predictedSet := make(map[string]struct{})
	for _, qt := range queryTokens {
		neighbors := provider.TopNeighbors(qt, cfg.NeighborsPerToken)
		for _, n := range neighbors {
			predictedSet[n] = struct{}{}
		}
	}
	// Exclude query tokens themselves from predicted (they're expected)
	for _, qt := range queryTokens {
		delete(predictedSet, qt)
	}

	// Build actual set from result tokens, excluding query tokens
	querySet := make(map[string]struct{}, len(queryTokens))
	for _, qt := range queryTokens {
		querySet[qt] = struct{}{}
	}
	actualSet := make(map[string]struct{})
	for _, rt := range resultTokens {
		if _, isQuery := querySet[rt]; !isQuery {
			actualSet[rt] = struct{}{}
		}
	}

	// Compute set differences
	var predicted, actual, onlyPredicted, onlyActual, overlap []string

	for tok := range predictedSet {
		predicted = append(predicted, tok)
		if _, inActual := actualSet[tok]; inActual {
			overlap = append(overlap, tok)
		} else {
			onlyPredicted = append(onlyPredicted, tok)
		}
	}

	for tok := range actualSet {
		actual = append(actual, tok)
		if _, inPredicted := predictedSet[tok]; !inPredicted {
			onlyActual = append(onlyActual, tok)
		}
	}

	// Jaccard distance: 1 - |intersection| / |union|
	unionSize := len(predictedSet) + len(actualSet) - len(overlap)
	var score float64
	if unionSize > 0 {
		score = 1.0 - float64(len(overlap))/float64(unionSize)
	}

	return PredictionError{
		Score:         score,
		Predicted:     predicted,
		Actual:        actual,
		OnlyPredicted: onlyPredicted,
		OnlyActual:    onlyActual,
		Overlap:       overlap,
	}
}
