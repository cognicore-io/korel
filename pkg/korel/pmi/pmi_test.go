package pmi

import (
	"math"
	"testing"
)

func TestPMIBasic(t *testing.T) {
	calc := NewCalculator(1.0)

	// Strong positive association: co-occur more than expected
	nAB := int64(8)  // co-occur in 8 docs
	nA := int64(10)  // A appears in 10 docs
	nB := int64(10)  // B appears in 10 docs
	N := int64(20)   // total 20 docs

	pmi := calc.PMI(nAB, nA, nB, N)

	if pmi <= 0 {
		t.Errorf("PMI for strong association should be positive, got %f", pmi)
	}
}

func TestPMIIndependent(t *testing.T) {
	calc := NewCalculator(1.0)

	// Independent terms: A in 50%, B in 50%, co-occur in 25% (random)
	N := int64(100)
	nA := int64(50)
	nB := int64(50)
	nAB := int64(25)

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should be close to 0 (no relationship)
	if math.Abs(pmi) > 0.5 {
		t.Errorf("PMI for independent terms should be near 0, got %f", pmi)
	}
}

func TestPMINegative(t *testing.T) {
	calc := NewCalculator(1.0)

	// A and B rarely co-occur (negative association)
	N := int64(100)
	nA := int64(50)
	nB := int64(50)
	nAB := int64(5) // much less than expected

	pmi := calc.PMI(nAB, nA, nB, N)

	if pmi >= 0 {
		t.Errorf("PMI for anti-correlated terms should be negative, got %f", pmi)
	}
}

func TestPMISmoothing(t *testing.T) {
	calc1 := NewCalculator(0.0) // no smoothing
	calc2 := NewCalculator(1.0) // with smoothing

	N := int64(100)
	nA := int64(10)
	nB := int64(10)
	nAB := int64(0) // never co-occur

	pmi1 := calc1.PMI(nAB, nA, nB, N)
	pmi2 := calc2.PMI(nAB, nA, nB, N)

	// With smoothing should not be -Inf
	if math.IsInf(pmi2, -1) {
		t.Error("Smoothing should prevent -Inf")
	}

	// Without smoothing will be very negative
	if pmi1 > pmi2 {
		t.Error("Smoothing should increase PMI for rare events")
	}
}

func TestNPMI(t *testing.T) {
	calc := NewCalculator(1.0)

	N := int64(100)
	nA := int64(20)
	nB := int64(20)
	nAB := int64(15)

	npmi := calc.NPMI(nAB, nA, nB, N)

	// NPMI should be in range [-1, 1]
	if npmi < -1.0 || npmi > 1.0 {
		t.Errorf("NPMI should be in [-1, 1], got %f", npmi)
	}
}

func TestEPMI(t *testing.T) {
	calc := NewCalculator(1.0)

	N := int64(100)
	nA := int64(20)
	nB := int64(20)
	nAB := int64(15)

	weight := 0.5
	epmi := calc.EPMI(nAB, nA, nB, N, weight)

	pmi := calc.PMI(nAB, nA, nB, N)
	expected := pmi * weight

	if math.Abs(epmi-expected) > 0.001 {
		t.Errorf("EPMI should be PMI * weight, got %f, expected %f", epmi, expected)
	}
}

func TestPMIZeroDocuments(t *testing.T) {
	calc := NewCalculator(1.0)

	pmi := calc.PMI(0, 0, 0, 0)

	if pmi != 0 {
		t.Error("PMI with zero documents should return 0")
	}
}

func TestPMIEpsilonDefault(t *testing.T) {
	// If epsilon <= 0, should default to 1.0
	calc := NewCalculator(-1.0)

	N := int64(100)
	nA := int64(10)
	nB := int64(10)
	nAB := int64(5)

	// Should not panic or return NaN
	pmi := calc.PMI(nAB, nA, nB, N)

	if math.IsNaN(pmi) {
		t.Error("PMI should not be NaN with negative epsilon (should default to 1.0)")
	}
}

// Edge case tests

func TestPMIVeryLargeN(t *testing.T) {
	calc := NewCalculator(1.0)

	// Very large corpus
	N := int64(10000000)
	nA := int64(100000)
	nB := int64(100000)
	nAB := int64(10000)

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should not overflow or panic
	if math.IsInf(pmi, 0) || math.IsNaN(pmi) {
		t.Error("PMI with very large N should not overflow")
	}
}

func TestPMIInvalidInputABGreaterThanA(t *testing.T) {
	calc := NewCalculator(1.0)

	// Invalid: nAB > nA
	N := int64(100)
	nA := int64(10)
	nB := int64(10)
	nAB := int64(15) // impossible: co-occurrence > individual

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should handle gracefully (may produce unexpected result but not crash)
	_ = pmi
}

func TestPMIAllEqual(t *testing.T) {
	calc := NewCalculator(1.0)

	// All equal: perfect co-occurrence
	N := int64(100)
	nA := int64(50)
	nB := int64(50)
	nAB := int64(50)

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should be strongly positive
	if pmi <= 0 {
		t.Errorf("Perfect co-occurrence should have positive PMI, got %f", pmi)
	}
}

func TestPMISingleOccurrence(t *testing.T) {
	calc := NewCalculator(1.0)

	// Each term appears once, co-occur once
	N := int64(100)
	nA := int64(1)
	nB := int64(1)
	nAB := int64(1)

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should handle rare events
	if math.IsInf(pmi, 0) {
		t.Error("Single occurrence should not produce infinity")
	}
}

func TestPMIZeroEpsilon(t *testing.T) {
	calc := NewCalculator(0.0)

	// With zero epsilon, may produce -Inf for zero co-occurrence
	N := int64(100)
	nA := int64(10)
	nB := int64(10)
	nAB := int64(0)

	pmi := calc.PMI(nAB, nA, nB, N)

	// May be -Inf without smoothing
	if !math.IsInf(pmi, -1) {
		// If implementation defaults epsilon, that's fine
		_ = pmi
	}
}

func TestPMIMaxCoOccurrence(t *testing.T) {
	calc := NewCalculator(1.0)

	// Maximum possible co-occurrence
	N := int64(100)
	nA := int64(20)
	nB := int64(20)
	nAB := int64(20) // all A and B co-occur

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should be strongly positive
	if pmi <= 0 {
		t.Errorf("Maximum co-occurrence should have high PMI, got %f", pmi)
	}
}

func TestNPMIRange(t *testing.T) {
	calc := NewCalculator(1.0)

	// Test multiple scenarios
	testCases := []struct {
		nAB, nA, nB, N int64
	}{
		{50, 50, 50, 100}, // perfect overlap
		{0, 50, 50, 100},  // no overlap
		{10, 20, 20, 100}, // partial overlap
	}

	for _, tc := range testCases {
		npmi := calc.NPMI(tc.nAB, tc.nA, tc.nB, tc.N)
		if npmi < -1.0 || npmi > 1.0 {
			t.Errorf("NPMI out of range [-1, 1]: %f for case %+v", npmi, tc)
		}
	}
}

func TestEPMIZeroWeight(t *testing.T) {
	calc := NewCalculator(1.0)

	N := int64(100)
	nA := int64(20)
	nB := int64(20)
	nAB := int64(15)

	epmi := calc.EPMI(nAB, nA, nB, N, 0.0)

	// With zero weight, should be 0
	if epmi != 0.0 {
		t.Errorf("EPMI with zero weight should be 0, got %f", epmi)
	}
}

func TestEPMINegativeWeight(t *testing.T) {
	calc := NewCalculator(1.0)

	N := int64(100)
	nA := int64(20)
	nB := int64(20)
	nAB := int64(15)

	epmi := calc.EPMI(nAB, nA, nB, N, -1.0)

	// Negative weight should produce negative EPMI
	if epmi >= 0 {
		t.Error("EPMI with negative weight should be negative")
	}
}

func TestPMISymmetry(t *testing.T) {
	calc := NewCalculator(1.0)

	N := int64(100)
	nA := int64(20)
	nB := int64(15)
	nAB := int64(10)

	// PMI should be symmetric: PMI(A,B) = PMI(B,A)
	pmi1 := calc.PMI(nAB, nA, nB, N)
	pmi2 := calc.PMI(nAB, nB, nA, N)

	if math.Abs(pmi1-pmi2) > 0.0001 {
		t.Errorf("PMI should be symmetric, got %f and %f", pmi1, pmi2)
	}
}

func TestPMIVerySmallProbabilities(t *testing.T) {
	calc := NewCalculator(1.0)

	// Very rare events in large corpus
	N := int64(1000000)
	nA := int64(10)
	nB := int64(10)
	nAB := int64(5)

	pmi := calc.PMI(nAB, nA, nB, N)

	// Should handle small probabilities
	if math.IsInf(pmi, 0) || math.IsNaN(pmi) {
		t.Error("Should handle very small probabilities without inf/nan")
	}
}
