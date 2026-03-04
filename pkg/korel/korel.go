package korel

import (
	"github.com/cognicore/korel/pkg/korel/inference"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store"
)

// Korel is the main knowledge engine facade
type Korel struct {
	store    store.Store
	pipeline *ingest.Pipeline
	inf      inference.Engine
	weights  ScoreWeights
	halfLife float64
	pmiCfg   pmi.Config
	graphCfg GraphConfig
	concepts map[string]struct{} // dictionary canonicals — used to filter PMI expansion
	rewriter *Rewriter           // optional query rewriter
}

// ScoreWeights defines the weights for hybrid scoring
type ScoreWeights struct {
	AlphaPMI     float64
	BetaCats     float64
	GammaRecency float64
	EtaAuthority float64
	DeltaLen     float64
	ThetaInfer   float64
	ZetaBM25     float64
	IotaTitle    float64
}

// GraphConfig controls edge generation thresholds in BuildGraph.
// All fields have sensible defaults via DefaultGraphConfig().
type GraphConfig struct {
	MinDF        int64   // minimum document frequency for a token to get edges (default: 3)
	MaxDFPercent float64 // tokens in more than this % of docs are excluded (default: 20.0)
	MinPMI       float64 // minimum NPMI score for a related_to edge (default: 0.3)
	MinTokenLen  int     // tokens shorter than this are excluded (default: 3)
}

// DefaultGraphConfig returns sensible defaults for graph edge generation.
func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		MinDF:        3,
		MaxDFPercent: 20.0,
		MinPMI:       0.3,
		MinTokenLen:  3,
	}
}

// DefaultScoreWeights returns production scoring weights.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		AlphaPMI:     1.0,
		BetaCats:     0.6,
		GammaRecency: 0.8,
		EtaAuthority: 0.2,
		DeltaLen:     0.05,
		ThetaInfer:   0.3,
		ZetaBM25:     0.8,
		IotaTitle:    5.0,
	}
}

// DefaultRecencyHalfLife is the default half-life (in days) for recency scoring.
const DefaultRecencyHalfLife = 14.0

// Options configures a Korel instance
type Options struct {
	Store           store.Store
	Pipeline        *ingest.Pipeline
	Inference       inference.Engine
	Weights         ScoreWeights
	RecencyHalfLife float64
	PMI             pmi.Config  // PMI computation settings (default: pmi.DefaultConfig())
	Graph           GraphConfig // Graph edge generation thresholds (default: DefaultGraphConfig())
	Rewriter        *Rewriter   // optional query rewriter (nil = disabled)
}

// New creates a Korel instance with the given dependencies.
// Zero-value Options fields fall back to sensible defaults.
func New(opts Options) *Korel {
	cfg := opts.PMI
	if cfg == (pmi.Config{}) {
		cfg = pmi.DefaultConfig()
	}
	gcfg := opts.Graph
	if gcfg == (GraphConfig{}) {
		gcfg = DefaultGraphConfig()
	}
	w := opts.Weights
	if w == (ScoreWeights{}) {
		w = DefaultScoreWeights()
	}
	hl := opts.RecencyHalfLife
	if hl == 0 {
		hl = DefaultRecencyHalfLife
	}
	var concepts map[string]struct{}
	if opts.Pipeline != nil {
		concepts = opts.Pipeline.KnownConcepts()
	}
	return &Korel{
		store:    opts.Store,
		pipeline: opts.Pipeline,
		inf:      opts.Inference,
		weights:  w,
		halfLife: hl,
		pmiCfg:   cfg,
		graphCfg: gcfg,
		concepts: concepts,
		rewriter: opts.Rewriter,
	}
}

// Close cleanly shuts down the Korel instance
func (k *Korel) Close() error {
	return k.store.Close()
}

// isConcept returns true if the token is a known dictionary concept.
// Only dictionary terms (multi-token phrases and their canonicals) qualify.
// This filters out noise words like "announced", "builds", "possible" that
// happen to co-occur with query terms.
func (k *Korel) isConcept(token string) bool {
	_, ok := k.concepts[token]
	return ok
}

func uniqueStrings(in []string) []string {
	set := make(map[string]struct{}, len(in))
	var out []string
	for _, val := range in {
		if val == "" {
			continue
		}
		if _, ok := set[val]; ok {
			continue
		}
		set[val] = struct{}{}
		out = append(out, val)
	}
	return out
}
