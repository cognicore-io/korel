package cards

import (
	"crypto/rand"
	"time"

	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/oklog/ulid/v2"
)

// Builder constructs explainable result cards
type Builder struct {
	entropy *ulid.MonotonicEntropy
}

// New creates a new card builder
func New() *Builder {
	return &Builder{
		entropy: ulid.Monotonic(rand.Reader, 0),
	}
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

// SourceRef references a source document
type SourceRef struct {
	URL  string
	Time time.Time
}

// PMIPair is a typed representation of a token co-occurrence pair with its PMI score.
type PMIPair struct {
	TokenA string
	TokenB string
	PMI    float64
}

// Explain provides transparency into retrieval
type Explain struct {
	QueryTokens     []string
	MatchedTokens   []string
	CategoryOverlap []string
	TopPairs        []PMIPair
}

// ScoredDoc represents a document with its score
type ScoredDoc struct {
	DocID       int64
	URL         string
	Title       string
	BodySnippet string
	Time        time.Time
	Tokens      []string
	Cats        []string
	Breakdown   rank.ScoreBreakdown
}

// Build creates a card from top-ranked documents
func (b *Builder) Build(title string, docs []ScoredDoc, query rank.Query, topPairs []PMIPair) Card {
	card := Card{
		ID:             ulid.MustNew(ulid.Now(), b.entropy).String(),
		Title:          title,
		Bullets:        make([]string, 0, len(docs)),
		Sources:        make([]SourceRef, 0, len(docs)),
		ScoreBreakdown: make(map[string]float64),
		Explain: Explain{
			QueryTokens:     query.Tokens,
			MatchedTokens:   []string{},
			CategoryOverlap: query.Categories,
			TopPairs:        topPairs,
		},
	}

	// Aggregate scores and collect only query-relevant tokens
	pmiSum, bm25Sum, titleSum, catsSum, recSum, authSum, lenSum := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	matchedTokens := make(map[string]struct{})
	queryTokenSet := make(map[string]struct{})
	for _, qt := range query.Tokens {
		queryTokenSet[qt] = struct{}{}
	}

	for _, doc := range docs {
		card.Bullets = append(card.Bullets, generateBullet(doc, queryTokenSet))

		// Add source reference
		card.Sources = append(card.Sources, SourceRef{
			URL:  doc.URL,
			Time: doc.Time,
		})

		// Aggregate breakdown
		pmiSum += doc.Breakdown.PMI
		bm25Sum += doc.Breakdown.BM25
		titleSum += doc.Breakdown.Title
		catsSum += doc.Breakdown.Cats
		recSum += doc.Breakdown.Recency
		authSum += doc.Breakdown.Authority
		lenSum += doc.Breakdown.Len

		// Collect only tokens that are relevant to the query
		for _, t := range doc.Tokens {
			if _, isQueryToken := queryTokenSet[t]; isQueryToken {
				matchedTokens[t] = struct{}{}
			}
		}
	}

	// Average scores
	n := float64(len(docs))
	if n > 0 {
		card.ScoreBreakdown["pmi"] = pmiSum / n
		card.ScoreBreakdown["bm25"] = bm25Sum / n
		card.ScoreBreakdown["title"] = titleSum / n
		card.ScoreBreakdown["cats"] = catsSum / n
		card.ScoreBreakdown["recency"] = recSum / n
		card.ScoreBreakdown["authority"] = authSum / n
		card.ScoreBreakdown["len"] = lenSum / n
	}

	// Convert matched tokens to slice
	card.Explain.MatchedTokens = make([]string, 0, len(matchedTokens))
	for t := range matchedTokens {
		card.Explain.MatchedTokens = append(card.Explain.MatchedTokens, t)
	}

	return card
}
