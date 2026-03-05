package korel

import (
	"context"
	"sort"
	"time"

	"github.com/cognicore/korel/pkg/korel/cards"
	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/cognicore/korel/pkg/korel/signals"
	"github.com/cognicore/korel/pkg/korel/store"
)

// SearchRequest defines a search query
type SearchRequest struct {
	Query         string
	TopK          int
	Now           time.Time
	EnableSignals bool // opt-in: compute Daimon-inspired self-monitoring signals

	// Feature 1: Intent-aware retrieval
	Mode SearchMode // auto-detected if empty

	// Feature 2: Temporal reasoning
	Since time.Time // zero = no lower bound
	Until time.Time // zero = no upper bound

	// Feature 3: Entity-centric answers
	Entity string // if set, focus results on this entity

	// Feature 4: Inference control
	MaxHops   int      // 0 = default (2), -1 = disable inference
	Relations []string // empty = all relations allowed

	// Feature 5: Feedback context
	SessionID string // for tracking feedback

	// Feature 7: Query rewriting
	EnableRewrite bool // opt-in query rewriting

	// Feature 8: Output format
	Format OutputFormat // default = cards
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
	TopPairs        []cards.PMIPair
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

	Intent    SearchMode    // detected search intent
	Rewritten string        // rewritten query (empty if unchanged)
	Evidence  []EvidenceScore // per-card evidence quality scores
	Formatted string        // non-empty when Format != FormatCards
}

// scored is a document with its computed score breakdown.
type scored struct {
	doc        store.Doc
	breakdown  rank.ScoreBreakdown
	totalScore float64
}

// Search executes a query and returns structured cards
func (k *Korel) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	// Feature 1: Detect intent
	mode := req.Mode
	if mode == ModeAuto {
		mode = DetectMode(req.Query)
	}

	// Feature 7: Query rewriting
	queryText := req.Query
	var rewritten string
	if req.EnableRewrite && k.rewriter != nil {
		rr := k.rewriter.Rewrite(req.Query)
		if rr.Rewritten != rr.Original {
			queryText = rr.Rewritten
			rewritten = rr.Rewritten
		}
	}

	// Process query through pipeline
	processed := k.pipeline.Process(queryText)

	// Feature 4: Inference control — determine hop budget
	maxHops := 2
	if req.MaxHops > 0 {
		maxHops = req.MaxHops
	}
	if req.MaxHops == -1 {
		maxHops = 0 // disable inference
	}

	// Expand via symbolic inference (uses fact graph populated by WarmInference)
	var inferExpanded []string
	if maxHops > 0 {
		type depthExpander interface {
			ExpandWithDepth(tokens []string, maxDepth, maxResults int) []string
		}
		if de, ok := k.inf.(depthExpander); ok {
			inferExpanded = de.ExpandWithDepth(processed.Tokens, maxHops, 50)
		} else {
			inferExpanded = k.inf.Expand(processed.Tokens)
		}
		// Feature 4: Filter by allowed relations if specified
		if len(req.Relations) > 0 {
			inferExpanded = k.filterByRelations(inferExpanded, processed.Tokens, req.Relations)
		}
	}
	expanded := uniqueStrings(append(processed.Tokens, inferExpanded...))

	// Capture inference proof chains from original tokens to expanded tokens.
	// Computed once and shared across all result cards.
	// Capped at 20 paths to bound latency on large rule graphs.
	var inferencePaths []InferencePath
	const maxInferencePaths = 20
	for _, qt := range processed.Tokens {
		if len(inferencePaths) >= maxInferencePaths {
			break
		}
		for _, et := range inferExpanded {
			if len(inferencePaths) >= maxInferencePaths {
				break
			}
			steps := k.inf.FindPath(qt, et)
			if len(steps) == 0 {
				continue
			}
			path := InferencePath{From: qt, To: et}
			for _, s := range steps {
				path.Steps = append(path.Steps, s.Rule)
			}
			inferencePaths = append(inferencePaths, path)
		}
	}

	// Expand via PMI co-occurrence statistics (corpus-driven discovery)
	// Skip when PMI weight is zero — no value in expanding if we won't score it.
	if k.weights.AlphaPMI > 0 {
		pmiExpanded := k.expandViaPMI(ctx, processed.Tokens)
		expanded = uniqueStrings(append(expanded, pmiExpanded...))
	}

	if req.TopK <= 0 {
		req.TopK = 3
	}

	if len(expanded) == 0 {
		return SearchResponse{Intent: mode, Rewritten: rewritten}, nil
	}

	// Feature 2: Temporal filtering — use time-bounded retrieval if Since/Until set
	var docs []store.Doc
	var err error
	if !req.Since.IsZero() || !req.Until.IsZero() {
		docs, err = k.store.GetDocsByTokensInRange(ctx, expanded, req.Since, req.Until, req.TopK*10)
	} else {
		docs, err = k.store.GetDocsByTokens(ctx, expanded, req.TopK*10)
	}
	if err != nil {
		return SearchResponse{}, err
	}

	// Feature 3: Entity focus — merge in entity-matching docs
	if req.Entity != "" {
		entityDocs, entityErr := k.store.GetDocsByEntity(ctx, "", req.Entity, req.TopK*10)
		if entityErr == nil && len(entityDocs) > 0 {
			docs = mergeDocSets(docs, entityDocs)
		}
	}

	if len(docs) == 0 {
		return SearchResponse{Intent: mode, Rewritten: rewritten}, nil
	}

	query := rank.Query{
		Tokens:     processed.Tokens,
		Categories: processed.Categories,
	}

	// Fetch corpus stats for BM25 scoring
	var corpusStats rank.CorpusStats
	if totalDocs, err := k.store.DocCount(ctx); err == nil {
		corpusStats.TotalDocs = totalDocs
	}
	if avgLen, err := k.store.AvgDocLen(ctx); err == nil {
		corpusStats.AvgDocLen = avgLen
	}

	// Feature 1: Mode-adjusted scoring weights
	mw := ModeWeights(mode)
	scorer := rank.NewScorer(rank.Weights{
		AlphaPMI:     k.weights.AlphaPMI * mw.PMI,
		BetaCats:     k.weights.BetaCats * mw.Cats,
		GammaRecency: k.weights.GammaRecency * mw.Recency,
		EtaAuthority: k.weights.EtaAuthority * mw.Authority,
		DeltaLen:     k.weights.DeltaLen * mw.Len,
		ZetaBM25:     k.weights.ZetaBM25,
		IotaTitle:    k.weights.IotaTitle,
	}, k.halfLife, rank.WithCorpusStats(corpusStats))

	// Collect all unique tokens from query + candidate docs for batch preloading.
	allTokens := make(map[string]struct{}, len(processed.Tokens)+len(docs)*20)
	for _, qt := range processed.Tokens {
		allTokens[qt] = struct{}{}
	}
	for _, doc := range docs {
		for _, dt := range doc.Tokens {
			allTokens[dt] = struct{}{}
		}
	}

	// Batch preload DF values for all tokens.
	tokenList := make([]string, 0, len(allTokens))
	for t := range allTokens {
		tokenList = append(tokenList, t)
	}
	dfMap, _ := k.store.GetTokenDFBatch(ctx, tokenList)
	if dfMap == nil {
		dfMap = make(map[string]int64)
	}
	dfFunc := func(token string) int64 {
		return dfMap[token]
	}

	// PMI scoring: only preload co-occurrence data when PMI weight > 0.
	var pmiFunc func(qt, dt string) float64
	dampingMap := make(map[string]float64, len(processed.Tokens))

	if k.weights.AlphaPMI > 0 {
		// Batch preload co-occurrence counts for query tokens vs all doc tokens.
		totalDocs, _ := k.store.DocCount(ctx)
		pmiCalc := k.getPMICalculator()
		coMap, _ := k.store.GetPairsBatch(ctx, processed.Tokens, tokenList)
		if coMap == nil {
			coMap = make(map[[2]string]int64)
		}
		pmiFunc = func(qt, dt string) float64 {
			if qt == dt {
				return 0
			}
			a, b := qt, dt
			if a > b {
				a, b = b, a
			}
			co := coMap[[2]string{a, b}]
			if co == 0 {
				return 0
			}
			dfA := dfMap[qt]
			dfB := dfMap[dt]
			if dfA == 0 || dfB == 0 || totalDocs == 0 {
				return 0
			}
			return pmiCalc(co, dfA, dfB, totalDocs)
		}

		// Compute damping map for query tokens based on neighbor counts.
		dampCfg := signals.DefaultDampingConfig()
		for _, qt := range processed.Tokens {
			neighbors, err := k.store.TopNeighbors(ctx, qt, 20)
			if err != nil {
				dampingMap[qt] = 1.0
				continue
			}
			vocabSize := len(docs) * 10
			dampingMap[qt] = signals.ComputeDamping(qt, &searchDensityProvider{
				neighborCount: len(neighbors),
				vocabSize:     vocabSize,
			}, dampCfg).DampingFactor
		}
	} else {
		// No PMI scoring — return 0 for all pairs.
		pmiFunc = func(qt, dt string) float64 { return 0 }
		for _, qt := range processed.Tokens {
			dampingMap[qt] = 1.0
		}
	}

	scoredDocs := make([]scored, 0, len(docs))
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	for _, doc := range docs {
		// Tokenize title for title-match scoring
		titleTokens := k.pipeline.Process(doc.Title).Tokens

		candidate := rank.Candidate{
			DocID:       doc.ID,
			Tokens:      doc.Tokens,
			TitleTokens: titleTokens,
			Categories:  doc.Cats,
			PublishedAt: doc.PublishedAt,
			LinksOut:    doc.LinksOut,
		}
		breakdown := scorer.ScoreWithOpts(query, candidate, now, pmiFunc, rank.ScoreOpts{
			DampingMap: dampingMap,
			DFFunc:     dfFunc,
		})
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
			DocID:       sdoc.doc.ID,
			URL:         sdoc.doc.URL,
			Title:       sdoc.doc.Title,
			BodySnippet: sdoc.doc.BodySnippet,
			Time:        sdoc.doc.PublishedAt,
			Tokens:      sdoc.doc.Tokens,
			Cats:        sdoc.doc.Cats,
			Breakdown:   sdoc.breakdown,
		}

		card := builder.Build(sdoc.doc.Title, []cards.ScoredDoc{scored}, query, nil)
		response.Cards = append(response.Cards, convertCard(card, expanded, inferencePaths))
	}

	// Feature 6: Evidence quality scoring
	for _, sdoc := range scoredDocs {
		ev := scoreEvidence([]scored{sdoc}, now, k.halfLife)
		if len(response.Cards) > 0 {
			ev.CardID = response.Cards[len(response.Evidence)].ID
		}
		response.Evidence = append(response.Evidence, ev)
	}

	// Feature 8: Output formatting
	if req.Format != "" && req.Format != FormatCards {
		response.Formatted = FormatOutput(req.Format, response.Cards)
	}

	response.Intent = mode
	response.Rewritten = rewritten

	// Compute self-monitoring signals (Daimon-inspired)
	if req.EnableSignals {
		response.Signals = k.computeSignals(ctx, processed.Tokens, scoredDocs)
	}

	return response, nil
}

// filterByRelations keeps only expanded tokens reachable through allowed relations.
func (k *Korel) filterByRelations(expanded, queryTokens, allowedRelations []string) []string {
	allowed := make(map[string]struct{}, len(allowedRelations))
	for _, r := range allowedRelations {
		allowed[r] = struct{}{}
	}
	var filtered []string
	for _, et := range expanded {
		for _, qt := range queryTokens {
			steps := k.inf.FindPath(qt, et)
			for _, s := range steps {
				if _, ok := allowed[s.Relation]; ok {
					filtered = append(filtered, et)
					goto next
				}
			}
		}
	next:
	}
	return uniqueStrings(filtered)
}

// mergeDocSets combines two document slices, deduplicating by ID.
func mergeDocSets(a, b []store.Doc) []store.Doc {
	seen := make(map[int64]struct{}, len(a))
	for _, d := range a {
		seen[d.ID] = struct{}{}
	}
	merged := make([]store.Doc, len(a))
	copy(merged, a)
	for _, d := range b {
		if _, ok := seen[d.ID]; !ok {
			merged = append(merged, d)
			seen[d.ID] = struct{}{}
		}
	}
	return merged
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

func convertCard(c cards.Card, expanded []string, paths []InferencePath) Card {
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
			InferencePaths:  paths,
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

// searchDensityProvider implements signals.DensityProvider for search-time damping.
type searchDensityProvider struct {
	neighborCount int
	vocabSize     int
}

func (p *searchDensityProvider) NeighborCount(_ string, _ float64) int { return p.neighborCount }
func (p *searchDensityProvider) VocabSize() int                       { return p.vocabSize }
