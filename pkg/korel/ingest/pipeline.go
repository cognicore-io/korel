package ingest

import "strings"

// Pipeline orchestrates the full ingestion flow:
// text → tokenization → multi-token recognition → taxonomy tagging
type Pipeline struct {
	tokenizer *Tokenizer
	parser    *MultiTokenParser
	taxonomy  *Taxonomy
}

// NewPipeline creates an ingestion pipeline with the given components
func NewPipeline(tokenizer *Tokenizer, parser *MultiTokenParser, taxonomy *Taxonomy) *Pipeline {
	return &Pipeline{
		tokenizer: tokenizer,
		parser:    parser,
		taxonomy:  taxonomy,
	}
}

// ProcessedDoc represents a document after ingestion processing
type ProcessedDoc struct {
	Tokens     []string
	Categories []string
	Entities   []Entity
}

// KnownConcepts returns the set of canonical dictionary terms.
func (p *Pipeline) KnownConcepts() map[string]struct{} {
	return p.parser.KnownConcepts()
}

// TaxonomySectors returns category → keywords for all sectors.
func (p *Pipeline) TaxonomySectors() map[string][]string {
	return p.taxonomy.sectors
}

// TaxonomyEvents returns category → keywords for all events.
func (p *Pipeline) TaxonomyEvents() map[string][]string {
	return p.taxonomy.events
}

// TaxonomyRegions returns category → keywords for all regions.
func (p *Pipeline) TaxonomyRegions() map[string][]string {
	return p.taxonomy.regions
}

// DictEntries returns all dictionary entries (canonical, variants, category).
func (p *Pipeline) DictEntries() []DictEntry {
	seen := make(map[string]struct{})
	var entries []DictEntry
	for _, e := range p.parser.dict {
		canonical := strings.ToLower(e.Canonical)
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		entries = append(entries, e)
	}
	return entries
}

// Process runs a document through the full ingestion pipeline
func (p *Pipeline) Process(text string) ProcessedDoc {
	// 1. Tokenize WITHOUT removing stopwords — multi-token phrases like
	// "open source" need all component words present for matching.
	raw := p.tokenizer.TokenizeKeepStopwords(text)

	// 2. Multi-token recognition (greedy longest match)
	tokens := p.parser.Parse(raw)

	// 3. Remove remaining stopwords (words not consumed by multi-token match)
	tokens = p.tokenizer.FilterStopwords(tokens)

	// 4. Taxonomy tagging
	categories := p.taxonomy.AssignCategories(tokens)
	entities := p.taxonomy.ExtractEntities(text)

	return ProcessedDoc{
		Tokens:     tokens,
		Categories: categories,
		Entities:   entities,
	}
}
