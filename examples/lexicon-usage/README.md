# Lexicon Integration Example

This example demonstrates the complete workflow for using Korel's lexicon system for synonym normalization and contextual token (c-token) relationships.

## What This Example Shows

1. **Corpus Analysis** - Analyze documents to discover contextual relationships
2. **C-Token Discovery** - Use skip-gram analysis to find related terms
3. **Lexicon Creation** - Build synonym maps and c-token relationships
4. **Tokenization with Normalization** - Apply lexicon during tokenization
5. **Query Expansion** - Use variants and c-tokens for comprehensive search
6. **Statistics Comparison** - See the impact of normalization

## Quick Run

```bash
cd examples/lexicon-usage
go build
./lexicon-usage
```

## Expected Output

```
=== Step 1: Analyzing corpus for c-token relationships ===
Processed 4 documents

=== Step 2: Discovering synonym candidates ===
Found 0 c-token pairs

Top c-token pairs (contextually related):

=== Step 3: Creating lexicon with synonyms ===
Loaded lexicon: 6 synonym groups, 15 total variants, 3 c-token entries

=== Step 4: Re-tokenizing with lexicon normalization ===

Comparison (without vs with lexicon):
  Unique tokens: 32 → 28 (reduced by normalization)

=== Step 5: Query expansion with c-tokens ===
Query: "game"
Normalized: "game"
Variants: [game games gaming]
C-tokens (contextually related): ai (PMI: 2.5)

=== Step 6: Stopword candidate analysis ===
High-DF tokens (potential stopwords):
  game - DF: 75.0%, PMIMax: 0.00

=== Integration complete ===
```

## What's Happening

### Step 1: Initial Analysis
- Creates an analyzer with skip-gram window (size=5)
- Processes 4 sample documents
- Tracks token co-occurrence within the window

### Step 2: C-Token Discovery
- Extracts high-PMI token pairs from skip-gram analysis
- These pairs suggest contextual relationships
- In this small example, corpus is too small for meaningful c-tokens

### Step 3: Lexicon Creation
The example creates synonym groups manually:
```go
lex.AddSynonymGroup("game", []string{"game", "games", "gaming"})
lex.AddSynonymGroup("ml", []string{"ml", "ML", "machine learning"})
lex.AddSynonymGroup("ai", []string{"ai", "AI", "artificial intelligence"})
```

And adds c-token relationships:
```go
lex.AddCToken("game", lexicon.CToken{Token: "ai", PMI: 2.5, Support: 10})
```

### Step 4: Re-tokenization
- Tokenizer is configured with the lexicon
- Documents are re-processed
- Variants normalize to canonical forms:
  - `"gaming"` → `"game"`
  - `"ML"` → `"ml"`
  - `"artificial intelligence"` → `"ai"`
- Result: Fewer unique tokens (32 → 28)

### Step 5: Query Expansion
When searching for "game":
- **Variants**: `["game", "games", "gaming"]` - finds all forms
- **C-tokens**: `["ai"]` - finds related concepts
- Combined: More comprehensive search results

### Step 6: Stopword Analysis
After normalization, re-analyze to find:
- High document frequency (DF) tokens
- Tokens with low PMI (appear everywhere, not meaningful)
- These become stopword candidates for next iteration

## Real-World Usage

In production, you would:

1. **Bootstrap from corpus**:
```bash
./bootstrap \
  --input corpus.jsonl \
  --domain ml \
  --output configs/ml \
  --synonym-limit 20 \
  --synonym-min-support 3 \
  --synonym-min-pmi 1.5
```

2. **Review generated synonyms.yaml**:
```yaml
synonyms:
  - canonical: ml
    variants: [ml, ML, machine learning]
    pmi: 3.21
    support: 45
```

3. **Load in your application**:
```go
lex, _ := lexicon.LoadFromYAML("configs/ml/synonyms.yaml")
tokenizer.SetLexicon(lex)
```

4. **Use for indexing and search**:
```go
// Indexing: normalize documents
tokens := tokenizer.Tokenize(document)
index.Add(docID, tokens)

// Searching: expand queries
query := "machine learning"
expanded := append(lex.Variants(query), extractCTokens(lex.GetCTokens(query))...)
results := index.Search(expanded)
```

## Key Concepts Demonstrated

### Synonym Normalization
- **Problem**: "game", "games", "gaming" counted separately
- **Solution**: All normalize to "game"
- **Benefit**: Better statistics, unified search

### C-Tokens (Contextual Tokens)
- **Discovery**: Skip-gram analysis finds co-occurring terms
- **Storage**: PMI and support scores quantify strength
- **Usage**: Query expansion finds semantically related documents

### Three-Stage Token Processing
1. **Clean**: Remove hyphens, normalize punctuation
2. **Normalize**: Apply lexicon (variants → canonical)
3. **Filter**: Remove stopwords

### Statistical Improvements
- Fewer unique tokens (reduced vocabulary)
- Higher token frequencies (better statistics)
- Stronger co-occurrence signals (better PMI)

## Corpus Size Considerations

This example uses 4 tiny documents, so:
- ❌ Not enough data for reliable c-token discovery
- ❌ PMI scores unstable with low support
- ✅ Good for demonstrating the workflow
- ✅ Shows API usage patterns

For production:
- **Minimum**: 100+ documents for stable PMI
- **Recommended**: 500+ documents for quality c-tokens
- **Ideal**: 1000+ documents for comprehensive coverage

## Related Documentation

- **[docs/LEXICON.md](../../docs/LEXICON.md)** - Complete lexicon system guide
- **[configs/synonyms.example.yaml](../../configs/synonyms.example.yaml)** - Synonym configuration template
- **[docs/BOOTSTRAP.md](../../docs/BOOTSTRAP.md)** - Bootstrap tool documentation

## Extending This Example

Try modifying the example to:

1. **Load from actual corpus**:
```go
items, _ := rss.LoadFromJSONL("corpus.jsonl")
for _, item := range items {
    tokens := tokenizer.Tokenize(item.Body)
    analyzer.Process(tokens, item.Categories)
}
```

2. **Save/load lexicon**:
```go
// After creating lexicon programmatically
// Save to YAML for reuse
data, _ := yaml.Marshal(lexiconData)
os.WriteFile("my-lexicon.yaml", data, 0644)

// Load it back
lex, _ := lexicon.LoadFromYAML("my-lexicon.yaml")
```

3. **Add more synonym groups**:
```go
lex.AddSynonymGroup("analyze", []string{"analyze", "analysis", "analytical"})
lex.AddSynonymGroup("database", []string{"database", "db", "databases"})
```

4. **Experiment with window sizes**:
```go
// Narrow context (bigram-like)
analyzer := analytics.NewAnalyzerWithWindow(2)

// Wide context (more relationships)
analyzer := analytics.NewAnalyzerWithWindow(10)
```

## Testing

The example serves as an integration test. To verify it works:

```bash
go test -v
# Should compile and run without errors
```

Or run directly:
```bash
go run main.go
# Should output the 6-step workflow
```

## Performance Notes

- **Lexicon lookup**: O(1) hash map lookups, very fast
- **Skip-gram tracking**: O(n × window) per document
- **Memory**: ~1-2MB for typical lexicons (1000s of entries)
- **Processing**: ~1000 docs/sec on laptop (with lexicon)

## Troubleshooting

**Q: Why are no c-tokens found?**
A: The example corpus is too small (4 docs). Use a larger corpus (100+ docs) for reliable c-token discovery.

**Q: Multi-token phrases don't work?**
A: The tokenizer processes words separately. For phrases like "machine learning", use the multi-token parser in the pipeline:
```go
pipeline := ingest.NewPipeline(tokenizer, ingest.NewMultiTokenParser(dict), taxonomy)
```

**Q: How do I know if normalization is working?**
A: Check the unique token count before/after. The example shows: 32 → 28 (12% reduction).

**Q: Can I use multiple lexicons?**
A: Currently one lexicon per tokenizer. For multiple domains, merge lexicons or use domain-specific tokenizers.

## Code Structure

```
main.go (186 lines)
├── Step 1: Initial corpus analysis (skip-gram tracking)
├── Step 2: C-token discovery (extract high-PMI pairs)
├── Step 3: Lexicon creation (add synonyms & c-tokens)
├── Step 4: Re-tokenization (with normalization)
├── Step 5: Query expansion (variants + c-tokens)
└── Step 6: Stopword analysis (high-DF, low-PMI)
```

## Next Steps

After understanding this example:

1. Run bootstrap on your own corpus
2. Review and edit the generated `synonyms.yaml`
3. Integrate lexicon into your indexing pipeline
4. Measure improvement in search relevance
5. Iterate: add more synonyms based on user queries

## License

Same as parent project (see root LICENSE file).
