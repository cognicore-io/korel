package korel

import (
	"context"

	"github.com/cognicore/korel/pkg/korel/analytics"
	"github.com/cognicore/korel/pkg/korel/autotune/rules"
	"github.com/cognicore/korel/pkg/korel/autotune/stopwords"
	"github.com/cognicore/korel/pkg/korel/autotune/taxonomy"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/signals"
	"github.com/cognicore/korel/pkg/korel/stoplist"
	"github.com/cognicore/korel/pkg/korel/store"
)

// AutoTuneResult contains suggestions from corpus analysis.
type AutoTuneResult struct {
	StopwordCandidates  []stoplist.Candidate  // cumulative from all iterations
	RuleSuggestions     []rules.Suggestion    // from final iteration stats
	TaxonomySuggestions []taxonomy.Suggestion // taxonomy drift suggestions (not auto-persisted)
	Iterations          []AutoTuneIteration   // per-round details
}

// AutoTuneIteration tracks one round of iterative autotune.
type AutoTuneIteration struct {
	Round          int
	NewStopwords   []string
	TotalStopwords int
}

// AutoTuneOptions configures the iterative autotune process.
type AutoTuneOptions struct {
	BaseStopwords  []string              // initial stopwords (from current pipeline)
	MaxIterations  int                   // max rounds (default: 3)
	Thresholds     stoplist.Thresholds   // stopword detection thresholds (default: AutoTuneDefaults())
	DisableDamping bool                  // set true to skip density-based damping (default: false = damping ON)
	DampingConfig  signals.DampingConfig // damping curve parameters (default: signals.DefaultDampingConfig())
}

// AutoTuneDefaults returns stoplist thresholds tuned for iterative autotune with NPMI.
// More relaxed than stoplist.DefaultThresholds() which targets news corpora.
func AutoTuneDefaults() stoplist.Thresholds {
	return stoplist.Thresholds{
		DFPercent:          50.0,
		PMIMax:             0.3,
		CatEntropy:         0.8,
		BootstrapDFPercent: 40.0,
		BootstrapEntropy:   0.4,
	}
}

// AutoTune performs iterative corpus analysis to discover stopwords and rules.
// Each round: tokenize with current stopwords → analyze stats → discover new
// stopwords → add them → repeat until convergence or MaxIterations.
func (k *Korel) AutoTune(ctx context.Context, texts []string, opts *AutoTuneOptions) (AutoTuneResult, error) {
	maxIter := 3
	var baseThresholds stoplist.Thresholds
	var currentStopwords []string
	enableDamping := true
	dampingCfg := signals.DefaultDampingConfig()
	if opts != nil {
		if opts.MaxIterations > 0 {
			maxIter = opts.MaxIterations
		}
		baseThresholds = opts.Thresholds
		currentStopwords = append(currentStopwords, opts.BaseStopwords...)
		if opts.DampingConfig != (signals.DampingConfig{}) {
			dampingCfg = opts.DampingConfig
		}
		enableDamping = !opts.DisableDamping
	}
	if baseThresholds == (stoplist.Thresholds{}) {
		baseThresholds = AutoTuneDefaults()
	}

	stopSet := make(map[string]struct{}, len(currentStopwords))
	for _, s := range currentStopwords {
		stopSet[s] = struct{}{}
	}

	var result AutoTuneResult
	var finalStats analytics.Stats

	// Build and process once. Subsequent rounds prune newly-discovered stopwords
	// instead of re-processing all texts from scratch.
	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer(currentStopwords),
		ingest.NewMultiTokenParser([]ingest.DictEntry{}),
		ingest.NewTaxonomy(),
	)
	analyzer := analytics.NewAnalyzer(k.pmiCfg)
	if enableDamping {
		analyzer.WithDamping(dampingCfg)
	}

	// Pre-tokenize all texts, then batch-process in parallel.
	docs := make([]analytics.DocTokens, len(texts))
	for i, text := range texts {
		processed := pipeline.Process(text)
		docs[i] = analytics.DocTokens{
			Tokens:     processed.Tokens,
			Categories: processed.Categories,
		}
	}
	analyzer.ProcessBatch(docs)

	for round := 0; round < maxIter; round++ {
		roundStats := analyzer.SnapshotView()

		// Detect whether corpus has categories.
		hasCats := false
		for _, cats := range roundStats.TokenCats {
			if len(cats) > 0 {
				hasCats = true
				break
			}
		}

		thresholds := baseThresholds
		if !hasCats {
			thresholds.CatEntropy = -1
		}

		// Fused computation: single pair iteration for both stopword stats and high PMI pairs.
		computed := roundStats.ComputeAll()

		// Discover stopword candidates using pre-computed stats.
		stopTuner := stopwords.AutoTuner{
			Provider:   &precomputedStopProvider{stats: computed.StopwordStats},
			Manager:    stoplist.NewManager(nil),
			Thresholds: thresholds,
		}
		candidates, err := stopTuner.Run(ctx)
		if err != nil {
			return result, err
		}

		// Filter to only genuinely new stopwords.
		var newStops []string
		for _, c := range candidates {
			if _, exists := stopSet[c.Token]; !exists {
				newStops = append(newStops, c.Token)
				stopSet[c.Token] = struct{}{}
				result.StopwordCandidates = append(result.StopwordCandidates, c)
			}
		}

		result.Iterations = append(result.Iterations, AutoTuneIteration{
			Round:          round + 1,
			NewStopwords:   newStops,
			TotalStopwords: len(stopSet),
		})

		if len(newStops) == 0 {
			break // converged
		}

		// Prune discovered stopwords from the analyzer instead of rebuilding.
		analyzer.RemoveTokens(newStops)
		currentStopwords = append(currentStopwords, newStops...)
	}

	finalStats = analyzer.Snapshot()

	// Rule suggestions from final iteration stats (fused computation).
	finalComputed := finalStats.ComputeAll()
	ruleTuner := rules.AutoTuner{
		Provider: &precomputedRuleProvider{pairs: finalComputed.HighPMIPairs},
	}
	suggestions, err := ruleTuner.Run(ctx)
	if err != nil {
		return result, err
	}
	result.RuleSuggestions = suggestions

	// Persist discovered stopwords to store.
	allStops := make([]string, 0, len(stopSet))
	for tok := range stopSet {
		allStops = append(allStops, tok)
	}
	if err := k.store.UpsertStoplist(ctx, allStops); err != nil {
		return result, err
	}

	// Persist high-confidence rules as edges and feed into inference engine.
	// Using source="autotune" so BuildGraph (which clears pmi/taxonomy/dict) preserves them.
	for _, s := range suggestions {
		if s.Confidence >= 0.6 {
			if err := k.store.UpsertEdge(ctx, store.Edge{
				Subject:  s.Subject,
				Relation: s.Relation,
				Object:   s.Object,
				Weight:   s.Confidence,
				Source:   "autotune",
			}); err != nil {
				return result, err
			}
			k.inf.AddFact(s.Relation, s.Subject, s.Object)
		}
	}

	// Taxonomy drift detection: compare corpus stats against current taxonomy.
	// Pass discovered stopwords so they don't appear as orphan candidates.
	if taxView := k.store.Taxonomy(); taxView != nil {
		flatTax := flattenTaxonomy(taxView)
		if len(flatTax) > 0 {
			driftStats := finalStats.TaxonomyDrift(flatTax, allStops)
			taxTuner := taxonomy.AutoTuner{
				Provider: &precomputedDriftProvider{stats: driftStats},
			}
			taxSuggestions, err := taxTuner.Run(ctx)
			if err != nil {
				return result, err
			}
			result.TaxonomySuggestions = taxSuggestions
		}
	}

	return result, nil
}

// flattenTaxonomy merges sectors, events, and regions into a single map.
func flattenTaxonomy(tv store.TaxonomyView) map[string][]string {
	flat := make(map[string][]string)
	for cat, keywords := range tv.AllSectors() {
		flat[cat] = append(flat[cat], keywords...)
	}
	for cat, keywords := range tv.AllEvents() {
		flat[cat] = append(flat[cat], keywords...)
	}
	for cat, keywords := range tv.AllRegions() {
		flat[cat] = append(flat[cat], keywords...)
	}
	return flat
}

// precomputedStopProvider implements stopwords.StatsProvider with pre-computed data.
type precomputedStopProvider struct {
	stats []stoplist.Stats
}

func (p *precomputedStopProvider) StopwordStats(ctx context.Context) ([]stoplist.Stats, error) {
	return p.stats, nil
}

// precomputedRuleProvider implements rules.StatsProvider with pre-computed data.
type precomputedRuleProvider struct {
	pairs []analytics.HighPMIPair
}

func (p *precomputedRuleProvider) HighPMIPairs(ctx context.Context) ([]rules.PairStats, error) {
	out := make([]rules.PairStats, len(p.pairs))
	for i, hp := range p.pairs {
		out[i] = rules.PairStats{
			Subject: hp.A,
			Object:  hp.B,
			PMI:     hp.PMI,
			Support: hp.Support,
		}
	}
	return out, nil
}

// precomputedDriftProvider implements taxonomy.StatsProvider with pre-computed data.
type precomputedDriftProvider struct {
	stats []taxonomy.DriftStats
}

func (p *precomputedDriftProvider) TaxonomyDrift(ctx context.Context) ([]taxonomy.DriftStats, error) {
	return p.stats, nil
}
