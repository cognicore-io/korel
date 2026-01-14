package pmi

import "math"

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
