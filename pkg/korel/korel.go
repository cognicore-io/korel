package korel

import (
	"context"
	"sort"
	"time"

	"github.com/cognicore/korel/pkg/korel/analytics"
	"github.com/cognicore/korel/pkg/korel/autotune/rules"
	"github.com/cognicore/korel/pkg/korel/autotune/stopwords"
	"github.com/cognicore/korel/pkg/korel/autotune/taxonomy"
	"github.com/cognicore/korel/pkg/korel/cards"
	"github.com/cognicore/korel/pkg/korel/inference"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/cognicore/korel/pkg/korel/signals"
	"github.com/cognicore/korel/pkg/korel/stoplist"
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
}

// ScoreWeights defines the weights for hybrid scoring
type ScoreWeights struct {
	AlphaPMI     float64
	BetaCats     float64
	GammaRecency float64
	EtaAuthority float64
	DeltaLen     float64
	ThetaInfer   float64
}

// Options configures a Korel instance
type Options struct {
	Store           store.Store
	Pipeline        *ingest.Pipeline
	Inference       inference.Engine
	Weights         ScoreWeights
	RecencyHalfLife float64
	PMI             pmi.Config // PMI computation settings (default: pmi.DefaultConfig())
}

// New creates a Korel instance with the given dependencies
func New(opts Options) *Korel {
	cfg := opts.PMI
	if cfg == (pmi.Config{}) {
		cfg = pmi.DefaultConfig()
	}
	return &Korel{
		store:    opts.Store,
		pipeline: opts.Pipeline,
		inf:      opts.Inference,
		weights:  opts.Weights,
		halfLife: opts.RecencyHalfLife,
		pmiCfg:   cfg,
	}
}

// Close cleanly shuts down the Korel instance
func (k *Korel) Close() error {
	return k.store.Close()
}

// RebuildPipeline constructs a new ingest pipeline from the store's current
// stoplist, dictionary, and taxonomy. Call after AutoTune to pick up newly
// discovered stopwords and rules.
func (k *Korel) RebuildPipeline() {
	var stopwords []string
	if sl := k.store.Stoplist(); sl != nil {
		stopwords = sl.AllStops()
	}

	var dictEntries []ingest.DictEntry
	if dv := k.store.Dict(); dv != nil {
		for _, e := range dv.AllEntries() {
			dictEntries = append(dictEntries, ingest.DictEntry{
				Canonical: e.Canonical,
				Category:  e.Category,
				Variants:  []string{e.Phrase},
			})
		}
	}

	tokenizer := ingest.NewTokenizer(stopwords)
	parser := ingest.NewMultiTokenParser(dictEntries)
	taxonomy := ingest.NewTaxonomy()

	if tv := k.store.Taxonomy(); tv != nil {
		for name, keywords := range tv.AllSectors() {
			taxonomy.AddSector(name, keywords)
		}
		for name, keywords := range tv.AllEvents() {
			taxonomy.AddEvent(name, keywords)
		}
		for name, keywords := range tv.AllRegions() {
			taxonomy.AddRegion(name, keywords)
		}
		for entType, names := range tv.AllEntities() {
			for name, keywords := range names {
				taxonomy.AddEntity(entType, name, keywords)
			}
		}
	}

	k.pipeline = ingest.NewPipeline(tokenizer, parser, taxonomy)
}

// IngestDoc represents a document to be ingested
type IngestDoc struct {
	URL         string
	Title       string
	Outlet      string
	PublishedAt time.Time
	BodyText    string
	SourceCats  []string
}

// Ingest processes and stores a document
func (k *Korel) Ingest(ctx context.Context, d IngestDoc) error {
	existingDoc, found, err := k.store.GetDocByURL(ctx, d.URL)
	if err != nil {
		return err
	}

	// Process through pipeline
	processed := k.pipeline.Process(d.BodyText)

	// Convert ingest.Entity to store.Entity
	storeEntities := make([]store.Entity, len(processed.Entities))
	for i, e := range processed.Entities {
		storeEntities[i] = store.Entity{
			Type:  e.Type,
			Value: e.Value,
		}
	}

	// Store document
	doc := store.Doc{
		URL:         d.URL,
		Title:       d.Title,
		Outlet:      d.Outlet,
		PublishedAt: d.PublishedAt,
		Cats:        uniqueStrings(append(d.SourceCats, processed.Categories...)),
		Ents:        storeEntities,
		Tokens:      processed.Tokens,
	}

	if err := k.store.UpsertDoc(ctx, doc); err != nil {
		return err
	}

	if found {
		if err := k.updateStats(ctx, existingDoc.Tokens, -1); err != nil {
			return err
		}
	}
	if err := k.updateStats(ctx, processed.Tokens, 1); err != nil {
		return err
	}

	return nil
}

// SearchRequest defines a search query
type SearchRequest struct {
	Query         string
	Cats          []string
	TopK          int
	Now           time.Time
	EnableSignals bool // opt-in: compute Daimon-inspired self-monitoring signals
}

// Card represents a structured, explainable result
type Card struct {
	ID             string
	Title          string
	Bullets        []string
	Sources        []SourceRef
	ScoreBreakdown map[string]float64
	Explain        Explain
}

// SourceRef is a reference to a source document
type SourceRef struct {
	URL  string
	Time time.Time
}

// Explain provides transparency into why a card was retrieved
type Explain struct {
	QueryTokens     []string
	ExpandedTokens  []string
	MatchedTokens   []string
	CategoryOverlap []string
	TopPairs        [][3]interface{}
	InferencePaths  []InferencePath
}

// InferencePath shows a chain of symbolic reasoning
type InferencePath struct {
	From  string
	To    string
	Steps []string
}

// SearchSignals contains self-monitoring signals computed during retrieval.
// Inspired by Daimon's cognitive architecture (Brian Jones).
type SearchSignals struct {
	// Collisions are concept pairs with high individual PMI but low joint PMI.
	// These are "thought collisions" — the query brought together concepts
	// the corpus hasn't connected. Potential discovery signal.
	Collisions []signals.Collision

	// PredictionError measures how much the retrieval results diverge from
	// what PMI statistics predicted. High score = surprising results.
	PredictionError signals.PredictionError

	// TokenDamping shows density-based damping for each query token.
	// Hub tokens that connect to too many concepts get dampened.
	TokenDamping []signals.TokenDensity
}

// SearchResponse contains search results
type SearchResponse struct {
	Cards   []Card
	Signals *SearchSignals // non-nil only when SearchRequest.EnableSignals is true
}

// scored is a document with its computed score breakdown.
type scored struct {
	doc        store.Doc
	breakdown  rank.ScoreBreakdown
	totalScore float64
}

// Search executes a query and returns structured cards
func (k *Korel) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	// Process query through pipeline
	processed := k.pipeline.Process(req.Query)

	// Expand via symbolic inference
	expanded := uniqueStrings(append(processed.Tokens, k.inf.Expand(processed.Tokens)...))

	if req.TopK <= 0 {
		req.TopK = 3
	}

	if len(expanded) == 0 {
		return SearchResponse{}, nil
	}

	// Retrieve candidate documents
	docs, err := k.store.GetDocsByTokens(ctx, expanded, req.TopK*4)
	if err != nil {
		return SearchResponse{}, err
	}
	if len(docs) == 0 {
		return SearchResponse{}, nil
	}

	query := rank.Query{
		Tokens:     processed.Tokens,
		Categories: processed.Categories,
	}

	scorer := rank.NewScorer(rank.Weights{
		AlphaPMI:     k.weights.AlphaPMI,
		BetaCats:     k.weights.BetaCats,
		GammaRecency: k.weights.GammaRecency,
		EtaAuthority: k.weights.EtaAuthority,
		DeltaLen:     k.weights.DeltaLen,
	}, k.halfLife)

	pmiFunc := func(qt, dt string) float64 {
		val, ok, err := k.store.GetPMI(ctx, qt, dt)
		if err != nil || !ok {
			return 0
		}
		return val
	}

	// Compute damping map for query tokens based on neighbor counts.
	dampingMap := make(map[string]float64, len(processed.Tokens))
	dampCfg := signals.DefaultDampingConfig()
	for _, qt := range processed.Tokens {
		neighbors, err := k.store.TopNeighbors(ctx, qt, 0) // 0 = get count only
		if err != nil {
			dampingMap[qt] = 1.0
			continue
		}
		density := signals.TokenDensity{
			Token:         qt,
			NeighborCount: len(neighbors),
			DampingFactor: 1.0,
		}
		// Estimate vocab size from number of unique tokens across candidate docs.
		vocabSize := len(docs) * 10 // rough estimate
		if vocabSize > 0 {
			density.DensityRatio = float64(len(neighbors)) / float64(vocabSize)
		}
		density.DampingFactor = signals.ComputeDamping(qt, &searchDensityProvider{
			neighborCount: len(neighbors),
			vocabSize:     vocabSize,
		}, dampCfg).DampingFactor
		dampingMap[qt] = density.DampingFactor
	}

	scoredDocs := make([]scored, 0, len(docs))
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	for _, doc := range docs {
		candidate := rank.Candidate{
			DocID:       doc.ID,
			Tokens:      doc.Tokens,
			Categories:  doc.Cats,
			PublishedAt: doc.PublishedAt,
			LinksOut:    doc.LinksOut,
		}
		breakdown := scorer.ScoreWithBreakdown(query, candidate, now, pmiFunc, dampingMap)
		scoredDocs = append(scoredDocs, scored{
			doc:        doc,
			breakdown:  breakdown,
			totalScore: breakdown.Total,
		})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].totalScore > scoredDocs[j].totalScore
	})
	if len(scoredDocs) > req.TopK {
		scoredDocs = scoredDocs[:req.TopK]
	}

	builder := cards.New()
	var response SearchResponse
	for _, sdoc := range scoredDocs {
		scored := cards.ScoredDoc{
			DocID:     sdoc.doc.ID,
			URL:       sdoc.doc.URL,
			Title:     sdoc.doc.Title,
			Time:      sdoc.doc.PublishedAt,
			Tokens:    sdoc.doc.Tokens,
			Cats:      sdoc.doc.Cats,
			Breakdown: sdoc.breakdown,
		}

		card := builder.Build(sdoc.doc.Title, []cards.ScoredDoc{scored}, query, nil)
		response.Cards = append(response.Cards, convertCard(card, expanded))
	}

	// Compute self-monitoring signals (Daimon-inspired)
	if req.EnableSignals {
		response.Signals = k.computeSignals(ctx, processed.Tokens, scoredDocs)
	}

	return response, nil
}

// computeSignals runs the three Daimon-inspired signal detectors on the search results.
func (k *Korel) computeSignals(ctx context.Context, queryTokens []string, scoredDocs []scored) *SearchSignals {
	// Shared adapter: converts store.TopNeighbors to signals.NeighborPMI
	topNeighborsFunc := func(c context.Context, token string, n int) ([]signals.NeighborPMI, error) {
		neighbors, err := k.store.TopNeighbors(c, token, n)
		if err != nil {
			return nil, err
		}
		result := make([]signals.NeighborPMI, len(neighbors))
		for i, nb := range neighbors {
			result[i] = signals.NeighborPMI{Token: nb.Token, PMI: nb.PMI}
		}
		return result, nil
	}

	sig := &SearchSignals{}

	// 1. Collision detection
	pmiLookup := &signals.StorePMILookup{
		Ctx:              ctx,
		GetPMI:           k.store.GetPMI,
		TopNeighborsFunc: topNeighborsFunc,
	}
	sig.Collisions = signals.DetectCollisions(queryTokens, pmiLookup, signals.DefaultCollisionConfig())

	// 2. Prediction error
	resultTokens := collectResultTokens(scoredDocs)
	neighborProvider := &signals.StoreNeighborProvider{
		Ctx:              ctx,
		TopNeighborsFunc: topNeighborsFunc,
	}
	sig.PredictionError = signals.ComputePredictionError(
		queryTokens, resultTokens, neighborProvider, signals.DefaultPredictionConfig(),
	)

	// 3. Density-based damping (uses TopNeighbors to count connections)
	densityProvider := &signals.StoreDensityProvider{
		Ctx:              ctx,
		TopNeighborsFunc: topNeighborsFunc,
		VocabSizeFunc: func(c context.Context) int {
			// Approximate vocab size from retrieved docs
			seen := make(map[string]struct{})
			for _, sd := range scoredDocs {
				for _, tok := range sd.doc.Tokens {
					seen[tok] = struct{}{}
				}
			}
			return len(seen)
		},
	}
	sig.TokenDamping = signals.ComputeDampingBatch(queryTokens, densityProvider, signals.DefaultDampingConfig())

	return sig
}

// collectResultTokens extracts unique tokens from all scored documents.
func collectResultTokens(docs []scored) []string {
	seen := make(map[string]struct{})
	for _, sd := range docs {
		for _, tok := range sd.doc.Tokens {
			seen[tok] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for tok := range seen {
		result = append(result, tok)
	}
	return result
}

func convertCard(c cards.Card, expanded []string) Card {
	card := Card{
		ID:             c.ID,
		Title:          c.Title,
		Bullets:        c.Bullets,
		ScoreBreakdown: c.ScoreBreakdown,
		Explain: Explain{
			QueryTokens:     c.Explain.QueryTokens,
			MatchedTokens:   c.Explain.MatchedTokens,
			CategoryOverlap: c.Explain.CategoryOverlap,
			TopPairs:        c.Explain.TopPairs,
			ExpandedTokens:  expanded,
		},
	}

	card.Sources = make([]SourceRef, len(c.Sources))
	for i, src := range c.Sources {
		card.Sources[i] = SourceRef{
			URL:  src.URL,
			Time: src.Time,
		}
	}

	return card
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

// AutoTuneResult contains suggestions from corpus analysis.
type AutoTuneResult struct {
	StopwordCandidates  []stoplist.Candidate  // cumulative from all iterations
	RuleSuggestions     []rules.Suggestion    // from final iteration stats
	TaxonomySuggestions []taxonomy.Suggestion // taxonomy drift suggestions (not auto-persisted)
	Iterations          []AutoTuneIteration   // per-round details
}

// AutoTuneIteration tracks one round of iterative autotune.
type AutoTuneIteration struct {
	Round         int
	NewStopwords  []string
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

	// Persist high-confidence rules as dict entries and feed into inference engine.
	for _, s := range suggestions {
		if s.Confidence >= 0.6 {
			phrase := s.Subject + " " + s.Object
			if err := k.store.UpsertDictEntry(ctx, phrase, phrase, s.Relation); err != nil {
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

// batchPairStore is an optional interface for stores that support batch pair operations.
type batchPairStore interface {
	BatchIncPairs(pairs [][2]string)
	BatchDecPairs(pairs [][2]string)
}

// searchDensityProvider implements signals.DensityProvider for search-time damping.
type searchDensityProvider struct {
	neighborCount int
	vocabSize     int
}

func (p *searchDensityProvider) NeighborCount(_ string, _ float64) int { return p.neighborCount }
func (p *searchDensityProvider) VocabSize() int                       { return p.vocabSize }

func (k *Korel) updateStats(ctx context.Context, tokens []string, delta int) error {
	if delta == 0 {
		return nil
	}
	unique := uniqueStrings(tokens)
	if len(unique) == 0 {
		return nil
	}

	for _, tok := range unique {
		current, err := k.store.GetTokenDF(ctx, tok)
		if err != nil {
			return err
		}
		newVal := current + int64(delta)
		if newVal < 0 {
			newVal = 0
		}
		if err := k.store.UpsertTokenDF(ctx, tok, newVal); err != nil {
			return err
		}
	}

	// Collect all pairs, then batch-write under a single lock if supported.
	n := len(unique)
	pairs := make([][2]string, 0, n*(n-1)/2)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			pairs = append(pairs, [2]string{unique[i], unique[j]})
		}
	}

	if bs, ok := k.store.(batchPairStore); ok {
		if delta > 0 {
			bs.BatchIncPairs(pairs)
		} else {
			bs.BatchDecPairs(pairs)
		}
	} else {
		for _, p := range pairs {
			var err error
			if delta > 0 {
				err = k.store.IncPair(ctx, p[0], p[1])
			} else {
				err = k.store.DecPair(ctx, p[0], p[1])
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}
