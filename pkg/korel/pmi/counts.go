package pmi

import "sort"

// Counter maintains co-occurrence counts for PMI calculation
type Counter struct {
	N   int64                   // total number of documents
	Nx  map[string]int64        // document frequency per token
	Nxy map[TokenPair]int64     // co-occurrence count per token pair
}

// TokenPair represents an ordered pair of tokens (t1 < t2)
type TokenPair struct {
	T1, T2 string
}

// NewCounter creates a new co-occurrence counter
func NewCounter() *Counter {
	return &Counter{
		N:   0,
		Nx:  make(map[string]int64),
		Nxy: make(map[TokenPair]int64),
	}
}

// AddDocument updates counts for a document with unique tokens
func (c *Counter) AddDocument(uniqueTokens []string) {
	c.N++

	// Update document frequency for each token
	for _, t := range uniqueTokens {
		c.Nx[t]++
	}

	// Update co-occurrence counts for all pairs
	sorted := make([]string, len(uniqueTokens))
	copy(sorted, uniqueTokens)
	sort.Strings(sorted)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pair := TokenPair{T1: sorted[i], T2: sorted[j]}
			c.Nxy[pair]++
		}
	}
}

// GetPairCount returns the co-occurrence count for a token pair
func (c *Counter) GetPairCount(t1, t2 string) int64 {
	// Ensure canonical ordering
	if t1 > t2 {
		t1, t2 = t2, t1
	}
	return c.Nxy[TokenPair{T1: t1, T2: t2}]
}

// GetTokenCount returns the document frequency for a token
func (c *Counter) GetTokenCount(t string) int64 {
	return c.Nx[t]
}

// TotalDocs returns the total number of documents processed
func (c *Counter) TotalDocs() int64 {
	return c.N
}

// UniqueTokens returns the number of unique tokens
func (c *Counter) UniqueTokens() int {
	return len(c.Nx)
}

// UniquePairs returns the number of unique token pairs
func (c *Counter) UniquePairs() int {
	return len(c.Nxy)
}
