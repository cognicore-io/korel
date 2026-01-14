package query

import (
	"context"
	"errors"
	"time"

	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/rank"
)

// Parser handles query parsing and normalization
type Parser struct {
	tokenizer *ingest.Tokenizer
	mtParser  *ingest.MultiTokenParser
	taxonomy  *ingest.Taxonomy
}

// NewParser creates a new query parser
func NewParser(tokenizer *ingest.Tokenizer, mtParser *ingest.MultiTokenParser, taxonomy *ingest.Taxonomy) *Parser {
	return &Parser{
		tokenizer: tokenizer,
		mtParser:  mtParser,
		taxonomy:  taxonomy,
	}
}

// Parse converts a raw query string into a structured Query
func (p *Parser) Parse(queryStr string) rank.Query {
	// Tokenize
	tokens := p.tokenizer.Tokenize(queryStr)

	// Multi-token recognition
	tokens = p.mtParser.Parse(tokens)

	// Assign categories
	cats := p.taxonomy.AssignCategories(tokens)

	return rank.Query{
		Tokens:     tokens,
		Categories: cats,
	}
}

// Retriever fetches candidate documents for a query
type Retriever struct {
	store Store
}

// Store interface for retrieval operations
type Store interface {
	GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]StoreDoc, error)
	TopNeighbors(ctx context.Context, token string, k int) ([]Neighbor, error)
}

// StoreDoc represents a document from the store
type StoreDoc struct {
	ID          int64
	URL         string
	Title       string
	PublishedAt time.Time
	Cats        []string
	LinksOut    int
	Tokens      []string
}

// Neighbor represents a PMI neighbor
type Neighbor struct {
	Token string
	PMI   float64
}

// NewRetriever creates a new retriever
func NewRetriever(store Store) *Retriever {
	return &Retriever{store: store}
}

// Retrieve fetches candidate documents based on query tokens and categories
//
// Strategy:
// 1. Exact token matches (token→docs index)
// 2. PMI neighbor expansion (token→neighbors→docs)
// 3. Deduplicate and return up to limit candidates
func (r *Retriever) Retrieve(ctx context.Context, q rank.Query, limit int) ([]rank.Candidate, error) {
	if r.store == nil {
		return nil, errors.New("retriever store is nil")
	}

	if limit <= 0 {
		limit = 100 // default
	}

	// Fetch docs matching query tokens directly
	docs, err := r.store.GetDocsByTokens(ctx, q.Tokens, limit*2)
	if err != nil {
		return nil, err
	}

	// Expand with PMI neighbors for better recall
	expandedTokens := make(map[string]bool)
	for _, tok := range q.Tokens {
		expandedTokens[tok] = true
		neighbors, err := r.store.TopNeighbors(ctx, tok, 5) // top 5 neighbors per token
		if err != nil {
			// Non-fatal: just skip expansion for this token
			continue
		}
		for _, n := range neighbors {
			expandedTokens[n.Token] = true
		}
	}

	// Fetch docs for expanded tokens
	expandedTokensList := make([]string, 0, len(expandedTokens))
	for tok := range expandedTokens {
		expandedTokensList = append(expandedTokensList, tok)
	}

	expandedDocs, err := r.store.GetDocsByTokens(ctx, expandedTokensList, limit*2)
	if err != nil {
		// Non-fatal: continue with original docs
		expandedDocs = []StoreDoc{}
	}

	// Merge and deduplicate
	seen := make(map[int64]bool)
	candidates := make([]rank.Candidate, 0, limit)

	for _, doc := range append(docs, expandedDocs...) {
		if seen[doc.ID] {
			continue
		}
		seen[doc.ID] = true

		candidates = append(candidates, rank.Candidate{
			DocID:       doc.ID,
			Tokens:      doc.Tokens,
			Categories:  doc.Cats,
			PublishedAt: doc.PublishedAt,
			LinksOut:    doc.LinksOut,
		})

		if len(candidates) >= limit {
			break
		}
	}

	return candidates, nil
}
