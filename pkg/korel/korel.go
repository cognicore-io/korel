package korel

import (
	"context"
	"sort"
	"time"

	"github.com/cognicore/korel/pkg/korel/cards"
	"github.com/cognicore/korel/pkg/korel/inference"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/cognicore/korel/pkg/korel/store"
)

// Korel is the main knowledge engine facade
type Korel struct {
	store    store.Store
	pipeline *ingest.Pipeline
	inf      inference.Engine
	weights  ScoreWeights
	halfLife float64
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
}

// New creates a Korel instance with the given dependencies
func New(opts Options) *Korel {
	return &Korel{
		store:    opts.Store,
		pipeline: opts.Pipeline,
		inf:      opts.Inference,
		weights:  opts.Weights,
		halfLife: opts.RecencyHalfLife,
	}
}

// Close cleanly shuts down the Korel instance
func (k *Korel) Close() error {
	return k.store.Close()
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
	Query string
	Cats  []string
	TopK  int
	Now   time.Time
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

// SearchResponse contains search results
type SearchResponse struct {
	Cards []Card
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

	type scored struct {
		doc        store.Doc
		breakdown  rank.ScoreBreakdown
		totalScore float64
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
		breakdown := scorer.ScoreWithBreakdown(query, candidate, now, pmiFunc)
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

	return response, nil
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

	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			var err error
			if delta > 0 {
				err = k.store.IncPair(ctx, unique[i], unique[j])
			} else if delta < 0 {
				err = k.store.DecPair(ctx, unique[i], unique[j])
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}
