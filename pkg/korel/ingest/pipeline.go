package ingest

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

// Process runs a document through the full ingestion pipeline
func (p *Pipeline) Process(text string) ProcessedDoc {
	// 1. Tokenize (remove stopwords, normalize case)
	tokens := p.tokenizer.Tokenize(text)

	// 2. Multi-token recognition (greedy longest match)
	tokens = p.parser.Parse(tokens)

	// 3. Taxonomy tagging
	categories := p.taxonomy.AssignCategories(tokens)
	entities := p.taxonomy.ExtractEntities(text)

	return ProcessedDoc{
		Tokens:     tokens,
		Categories: categories,
		Entities:   entities,
	}
}
