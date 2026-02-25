package signals

import "math"

// TokenDensity describes how broadly connected a token is in the PMI graph.
//
// Inspired by Daimon's density invariant: HDM vectors are kept at 40-60% activation
// to prevent drift. Korel's self-adjusting stoplist is a binary version of this idea
// (stop/not stop). Density-based damping generalizes it to a continuous curve:
// tokens connecting to too many other tokens get dampened, not removed.
type TokenDensity struct {
	Token         string
	NeighborCount int     // number of tokens with PMI > threshold
	TotalVocab    int     // total vocabulary size
	DensityRatio  float64 // neighborCount / totalVocab (0..1)
	DampingFactor float64 // 1.0 = no damping, approaching 0.0 = fully dampened
}

// DampingConfig controls the density-based damping curve.
type DampingConfig struct {
	// PMIThreshold is the minimum PMI for counting a neighbor.
	// Only pairs above this threshold count toward the connection density.
	// Default: 0.1
	PMIThreshold float64

	// LowDensity is the density ratio below which no damping is applied.
	// Tokens connecting to fewer than this fraction of vocabulary are fine.
	// Default: 0.3 (30% of vocab)
	LowDensity float64

	// HighDensity is the density ratio at which damping reaches maximum.
	// Tokens connecting to this fraction or more get maximum damping.
	// Default: 0.6 (60% of vocab, matching Daimon's invariant)
	HighDensity float64

	// MinFactor is the minimum damping factor (floor).
	// Even the most-connected terms keep at least this weight.
	// Default: 0.1 (retain 10% weight, never fully zero)
	MinFactor float64
}

// DefaultDampingConfig returns sensible defaults based on Daimon's density invariant.
// PMIThreshold uses NPMI scale [-1,1] by default.
func DefaultDampingConfig() DampingConfig {
	return DampingConfig{
		PMIThreshold: 0.05,
		LowDensity:   0.3,
		HighDensity:  0.6,
		MinFactor:    0.1,
	}
}

// DensityProvider returns connection density data for a token.
type DensityProvider interface {
	// NeighborCount returns how many tokens have PMI > threshold with the given token.
	NeighborCount(token string, pmiThreshold float64) int

	// VocabSize returns the total number of unique tokens in the corpus.
	VocabSize() int
}

// ComputeDamping calculates the damping factor for a single token.
//
// The curve is a smooth sigmoid between LowDensity (factor=1.0) and
// HighDensity (factor=MinFactor). Tokens below LowDensity are untouched.
// Tokens above HighDensity are dampened to MinFactor.
func ComputeDamping(token string, provider DensityProvider, cfg DampingConfig) TokenDensity {
	if provider == nil {
		return TokenDensity{Token: token, DampingFactor: 1.0}
	}

	count := provider.NeighborCount(token, cfg.PMIThreshold)
	vocab := provider.VocabSize()

	var ratio float64
	if vocab > 0 {
		ratio = float64(count) / float64(vocab)
	}

	factor := dampingCurve(ratio, cfg)

	return TokenDensity{
		Token:         token,
		NeighborCount: count,
		TotalVocab:    vocab,
		DensityRatio:  ratio,
		DampingFactor: factor,
	}
}

// ComputeDampingBatch calculates damping for multiple tokens.
func ComputeDampingBatch(tokens []string, provider DensityProvider, cfg DampingConfig) []TokenDensity {
	result := make([]TokenDensity, len(tokens))
	for i, tok := range tokens {
		result[i] = ComputeDamping(tok, provider, cfg)
	}
	return result
}

// dampingCurve maps a density ratio to a damping factor using a smooth curve.
//
// Below LowDensity: factor = 1.0 (no damping)
// Above HighDensity: factor = MinFactor (maximum damping)
// Between: smooth sigmoid transition
func dampingCurve(ratio float64, cfg DampingConfig) float64 {
	if ratio <= cfg.LowDensity {
		return 1.0
	}
	if ratio >= cfg.HighDensity {
		return cfg.MinFactor
	}

	// Normalized position in the transition zone (0..1)
	t := (ratio - cfg.LowDensity) / (cfg.HighDensity - cfg.LowDensity)

	// Smoothstep (Hermite interpolation) for natural transition
	smooth := t * t * (3.0 - 2.0*t)

	return 1.0 - smooth*(1.0-cfg.MinFactor)
}

// DampedPMI applies density-based damping to a PMI score.
// The damping factor reduces the contribution of hub terms
// that connect to too many concepts (they carry less information).
func DampedPMI(pmi float64, density TokenDensity) float64 {
	return pmi * density.DampingFactor
}

// DampedPMIPair applies damping from both tokens in a pair.
// Uses the geometric mean of both damping factors so that
// a pair where either token is a hub gets dampened.
func DampedPMIPair(pmi float64, densityA, densityB TokenDensity) float64 {
	combinedFactor := math.Sqrt(densityA.DampingFactor * densityB.DampingFactor)
	return pmi * combinedFactor
}
