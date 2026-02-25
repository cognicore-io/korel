package signals

import "context"

// StorePMILookup adapts a PMI store to the PMILookup interface used by
// collision detection. The store must provide joint PMI and top neighbors.
type StorePMILookup struct {
	Ctx context.Context

	// GetPMI returns the PMI between two tokens.
	GetPMI func(ctx context.Context, a, b string) (float64, bool, error)

	// TopNeighborsFunc returns the top-k neighbors with PMI scores.
	TopNeighborsFunc func(ctx context.Context, token string, k int) ([]NeighborPMI, error)

	// PMIMaxK is how many neighbors to fetch to approximate PMIMax.
	// Default: 1 (the top neighbor's PMI is a good proxy).
	PMIMaxK int
}

// NeighborPMI is a token with its PMI score, returned by the store adapter.
type NeighborPMI struct {
	Token string
	PMI   float64
}

func (s *StorePMILookup) PMIMax(token string) float64 {
	k := s.PMIMaxK
	if k <= 0 {
		k = 1
	}
	neighbors, err := s.TopNeighborsFunc(s.Ctx, token, k)
	if err != nil || len(neighbors) == 0 {
		return 0
	}
	return neighbors[0].PMI
}

func (s *StorePMILookup) JointPMI(a, b string) float64 {
	val, ok, err := s.GetPMI(s.Ctx, a, b)
	if err != nil || !ok {
		return 0
	}
	return val
}

// StoreNeighborProvider adapts a store to the NeighborProvider interface
// used by prediction error computation.
type StoreNeighborProvider struct {
	Ctx              context.Context
	TopNeighborsFunc func(ctx context.Context, token string, k int) ([]NeighborPMI, error)
}

func (s *StoreNeighborProvider) TopNeighbors(token string, k int) []string {
	neighbors, err := s.TopNeighborsFunc(s.Ctx, token, k)
	if err != nil {
		return nil
	}
	result := make([]string, len(neighbors))
	for i, n := range neighbors {
		result[i] = n.Token
	}
	return result
}

// StoreDensityProvider adapts a store to the DensityProvider interface
// used by density-based damping.
type StoreDensityProvider struct {
	Ctx context.Context

	// TopNeighborsFunc returns neighbors — we count how many have PMI > threshold.
	TopNeighborsFunc func(ctx context.Context, token string, k int) ([]NeighborPMI, error)

	// VocabSizeFunc returns the total vocabulary size.
	VocabSizeFunc func(ctx context.Context) int

	// MaxNeighbors is the maximum number of neighbors to fetch for counting.
	// Should be large enough to capture the full neighborhood. Default: 500.
	MaxNeighbors int
}

func (s *StoreDensityProvider) NeighborCount(token string, pmiThreshold float64) int {
	k := s.MaxNeighbors
	if k <= 0 {
		k = 500
	}
	neighbors, err := s.TopNeighborsFunc(s.Ctx, token, k)
	if err != nil {
		return 0
	}
	count := 0
	for _, n := range neighbors {
		if n.PMI >= pmiThreshold {
			count++
		}
	}
	return count
}

func (s *StoreDensityProvider) VocabSize() int {
	return s.VocabSizeFunc(s.Ctx)
}
