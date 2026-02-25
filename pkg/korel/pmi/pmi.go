package pmi

import "math"

// Config controls PMI computation across the system.
type Config struct {
	Epsilon float64 // Laplace smoothing constant (default: 1.0)
	MinDF   int64   // Minimum document frequency for neighbor candidates (default: 5)
	UseNPMI bool    // Use normalized PMI [-1,1] instead of raw PMI (default: true)
}

// DefaultConfig returns the default PMI configuration.
// NPMI is enabled by default for better normalization across different corpus sizes.
func DefaultConfig() Config {
	return Config{
		Epsilon: 1.0,
		MinDF:   5,
		UseNPMI: true,
	}
}

// Calculator handles PMI (Pointwise Mutual Information) calculations
type Calculator struct {
	epsilon float64 // smoothing constant
}

// NewCalculator creates a new PMI calculator with the given epsilon
func NewCalculator(epsilon float64) *Calculator {
	if epsilon <= 0 {
		epsilon = 1.0
	}
	return &Calculator{epsilon: epsilon}
}

// NewCalculatorFromConfig creates a Calculator from a Config.
func NewCalculatorFromConfig(cfg Config) *Calculator {
	eps := cfg.Epsilon
	if eps <= 0 {
		eps = DefaultConfig().Epsilon
	}
	return NewCalculator(eps)
}

// Score computes PMI or NPMI based on the useNPMI flag.
func (c *Calculator) Score(nAB, nA, nB, N int64, useNPMI bool) float64 {
	if useNPMI {
		return c.NPMI(nAB, nA, nB, N)
	}
	return c.PMI(nAB, nA, nB, N)
}

// PMI calculates the pointwise mutual information between two tokens
//
// PMI(a,b) = log((N_ab + ε) * N / ((N_a + ε)(N_b + ε)))
//
// Where:
//   - N_ab = number of documents containing both a and b
//   - N_a, N_b = number of documents containing each token
//   - N = total number of documents
//   - ε = smoothing constant (default 1.0)
func (c *Calculator) PMI(nAB, nA, nB, N int64) float64 {
	if N == 0 {
		return 0
	}

	numerator := (float64(nAB) + c.epsilon) * float64(N)
	denominator := (float64(nA) + c.epsilon) * (float64(nB) + c.epsilon)

	if denominator == 0 {
		return 0
	}

	return math.Log(numerator / denominator)
}

// NPMI calculates normalized PMI (range: -1 to 1)
// NPMI(a,b) = PMI(a,b) / -log(P(a,b))
func (c *Calculator) NPMI(nAB, nA, nB, N int64) float64 {
	if N == 0 || nAB == 0 {
		return 0
	}

	pmi := c.PMI(nAB, nA, nB, N)
	pAB := (float64(nAB) + c.epsilon) / float64(N)
	logPAB := math.Log(pAB)

	if logPAB == 0 {
		return 0
	}

	return pmi / -logPAB
}

// EPMI calculates enhanced PMI with additional weighting
// This can include time decay, authority, or other domain-specific factors
func (c *Calculator) EPMI(nAB, nA, nB, N int64, weight float64) float64 {
	return c.PMI(nAB, nA, nB, N) * weight
}
