package stoplist

// Manager handles self-adjusting stopword management
type Manager struct {
	stops map[string]Reason
}

// Reason explains why a token is a stopword
type Reason struct {
	HighDF      bool    // high document frequency
	LowPMI      bool    // low PMI with all other tokens
	HighEntropy bool    // uniform distribution across categories
	IDF         float64 // inverse document frequency
	PMIMax      float64 // maximum PMI with any token
	CatEntropy  float64 // category entropy
}

// NewManager creates a new stoplist manager
func NewManager(initialStops []string) *Manager {
	stops := make(map[string]Reason, len(initialStops))
	for _, s := range initialStops {
		stops[s] = Reason{}
	}
	return &Manager{stops: stops}
}

// IsStop checks if a token is a stopword
func (m *Manager) IsStop(token string) bool {
	_, ok := m.stops[token]
	return ok
}

// Add adds a token to the stoplist with a reason
func (m *Manager) Add(token string, reason Reason) {
	m.stops[token] = reason
}

// Remove removes a token from the stoplist
func (m *Manager) Remove(token string) {
	delete(m.stops, token)
}

// All returns all stopwords
func (m *Manager) All() []string {
	result := make([]string, 0, len(m.stops))
	for s := range m.stops {
		result = append(result, s)
	}
	return result
}

// Stats holds statistics for candidate evaluation
type Stats struct {
	Token      string
	DF         int64
	DFPercent  float64
	IDF        float64
	PMIMax     float64
	CatEntropy float64
}

// Candidate represents a candidate stopword
type Candidate struct {
	Token  string
	Reason Reason
	Score  float64 // confidence score
}

// SuggestCandidates suggests tokens that should be stopwords
func (m *Manager) SuggestCandidates(stats []Stats, thresholds Thresholds) []Candidate {
	var candidates []Candidate

	if thresholds.BootstrapDFPercent == 0 {
		thresholds.BootstrapDFPercent = 60
	}
	if thresholds.BootstrapEntropy == 0 {
		thresholds.BootstrapEntropy = 0.4
	}

	for _, s := range stats {
		if m.IsStop(s.Token) {
			continue // already a stopword
		}

		bootstrap := s.PMIMax == 0
		highDF := s.DFPercent > thresholds.DFPercent
		lowPMI := s.PMIMax < thresholds.PMIMax || bootstrap
		highEntropy := s.CatEntropy > thresholds.CatEntropy
		bootstrapDF := s.DFPercent > thresholds.BootstrapDFPercent
		bootstrapEntropy := s.CatEntropy == 0 || s.CatEntropy > thresholds.BootstrapEntropy

		reason := Reason{
			HighDF:      highDF || (bootstrap && bootstrapDF),
			LowPMI:      lowPMI,
			HighEntropy: highEntropy || (bootstrap && (bootstrapEntropy || bootstrapDF)),
			IDF:         s.IDF,
			PMIMax:      s.PMIMax,
			CatEntropy:  s.CatEntropy,
		}

		meets := reason.HighDF && reason.LowPMI && reason.HighEntropy
		if bootstrap {
			meets = bootstrapDF && (bootstrapEntropy || thresholds.BootstrapEntropy <= 0)
			if !meets && bootstrapDF {
				// DF-only fallback for bootstrap mode
				meets = true
			}
		}

		if meets {
			entropyComponent := s.CatEntropy
			if bootstrap && entropyComponent < thresholds.BootstrapEntropy {
				entropyComponent = thresholds.BootstrapEntropy
			}
			score := (s.DFPercent/100.0 + (1.0 - s.PMIMax) + entropyComponent) / 3.0
			candidates = append(candidates, Candidate{
				Token:  s.Token,
				Reason: reason,
				Score:  score,
			})
		}
	}

	return candidates
}

// Thresholds defines criteria for stopword identification
type Thresholds struct {
	DFPercent  float64 // e.g., 80% - appears in 80% of documents
	PMIMax     float64 // e.g., 0.1 - maximum PMI with any token
	CatEntropy float64 // e.g., 0.8 - high entropy across categories
	// Bootstrap thresholds are used when PMI data is unavailable (PMIMax == 0)
	BootstrapDFPercent float64
	BootstrapEntropy   float64
}

// DefaultThresholds returns sensible default thresholds.
// PMIMax uses NPMI scale [-1,1] by default.
func DefaultThresholds() Thresholds {
	return Thresholds{
		DFPercent:          80.0,
		PMIMax:             0.15,
		CatEntropy:         0.8,
		BootstrapDFPercent: 60.0,
		BootstrapEntropy:   0.4,
	}
}
