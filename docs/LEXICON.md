# Lexicon System: Synonyms and Contextual Tokens (C-Tokens)

## Overview

The lexicon system provides corpus-specific vocabulary normalization and semantic relationships for improved information retrieval. It consists of three integrated components:

1. **Synonym Maps**: Normalize variant forms to canonical tokens
2. **C-Tokens (Contextual Tokens)**: Track semantically related terms via skip-gram co-occurrence
3. **Bootstrap Integration**: Auto-generate lexicon candidates from corpus analysis

## Table of Contents

- [Core Concepts](#core-concepts)
- [Architecture](#architecture)
- [Usage Workflow](#usage-workflow)
- [Bootstrap Tool](#bootstrap-tool)
- [API Reference](#api-reference)
- [Examples](#examples)

## Core Concepts

### Synonym Maps

**Problem**: Token variants fragment statistics and reduce retrieval effectiveness.
- "game", "games", "gaming", "gamer" are counted as separate tokens
- Search for "machine learning" misses "ML" and vice versa
- Morphological variants reduce statistical power

**Solution**: Bidirectional mapping between variants and canonical forms.
- **Normalization**: `"gaming"` → `"game"` (for indexing/analysis)
- **Expansion**: `"game"` → `["game", "games", "gaming", "gamer"]` (for query expansion)

### C-Tokens (Contextual Tokens)

**Problem**: Related concepts should boost each other's relevance, but simple keyword matching misses semantic relationships.

**Solution**: Track tokens that frequently co-occur within a configurable window (default: 5 positions).
- Skip-gram analysis identifies contextually related terms
- PMI (Pointwise Mutual Information) measures association strength
- Used for query expansion and explainability

**Example**:
```
"transformer attention mechanism"
→ C-tokens: (transformer, attention), (transformer, mechanism), (attention, mechanism)
→ PMI scores indicate strength of association
```

**Distinction from bigrams**:
- **Bigrams**: Adjacent tokens only (`"deep learning"` as a phrase)
- **Skip-grams**: Window-based co-occurrence (`"deep"` and `"network"` within 5 tokens)
- **C-tokens**: Semantically related, not necessarily adjacent

### Skip-Gram Configuration

**Window Size**: Controls context distance for c-token tracking
- `DefaultSkipGramWindow = 5`: Default context window
- `MinSkipGramWindow = 2`: Minimum valid window size
- Configurable via `analytics.NewAnalyzerWithWindow(windowSize)`

**Example with window=3**:
```
Tokens: ["deep", "learning", "neural", "network"]

Skip-gram pairs captured:
  (deep, learning)  - distance 1
  (deep, neural)    - distance 2
  (learning, neural) - distance 1
  (learning, network) - distance 2
  (neural, network)  - distance 1

Not captured (outside window=3):
  (deep, network)   - distance 3 (at boundary, excluded)
```

## Architecture

### Package Structure

```
pkg/korel/
├── lexicon/           # Core lexicon management
│   ├── lexicon.go     # Synonym maps, c-tokens, normalization
│   └── lexicon_test.go
├── analytics/         # Skip-gram tracking and PMI calculation
│   ├── analyzer.go    # Document analysis with skip-gram windows
│   └── analyzer_test.go
├── ingest/           # Tokenization with lexicon integration
│   ├── tokenizer.go   # 3-stage: clean → normalize → filter
│   └── tokenizer_test.go
cmd/bootstrap/         # Auto-generate lexicon from corpus
    └── main.go        # Synonym candidate discovery
```

### Data Flow

```
Corpus Documents
    ↓
[Tokenizer] ────→ Basic tokens
    ↓ (no lexicon yet)
[Analyzer with skip-grams]
    ↓
Statistics (TokenDF, SkipGramCounts, PMI)
    ↓
[Bootstrap Tool]
    ↓
synonyms.yaml (candidates for review)
    ↓ (manual editing)
[Lexicon.LoadFromYAML]
    ↓
Lexicon (synonym maps + c-tokens)
    ↓
[Tokenizer.SetLexicon] ────→ Normalized tokens
    ↓
[Analyzer] ────→ Improved statistics
    ↓
[Index/Search with query expansion]
```

## Usage Workflow

### 1. Bootstrap: Auto-Generate Lexicon

Run corpus analysis to discover synonym candidates:

```bash
./bootstrap \
  --input corpus.jsonl \
  --domain "machine-learning" \
  --output configs/ml-domain \
  --synonym-limit 20 \
  --synonym-min-support 3 \
  --synonym-min-pmi 1.5
```

Output files:
- `synonyms.yaml` - Synonym candidates (requires manual review)
- `stoplist.yaml` - Stopword suggestions
- `tokens.dict` - Multi-token phrases
- `taxonomies.yaml` - Category keywords
- `bootstrap-report.json` - Full statistics

### 2. Review and Edit Synonyms

Edit `synonyms.yaml` to:
- ✅ Keep true synonyms: `game/games/gaming`
- ❌ Remove collocations: `machine/learning` (appear together, not synonyms)
- ❌ Remove false positives from c-token clustering

Example `synonyms.yaml`:
```yaml
synonyms:
  - canonical: ml
    variants: [ml, ML, machine learning, machine-learning]
    pmi: 3.21
    support: 45

  - canonical: game
    variants: [game, games, gaming, gamer, gamers]
    pmi: 2.85
    support: 67
```

### 3. Load Lexicon in Code

```go
import (
    "github.com/cognicore/korel/pkg/korel/lexicon"
    "github.com/cognicore/korel/pkg/korel/ingest"
)

// Load synonyms from YAML
lex, err := lexicon.LoadFromYAML("configs/synonyms.yaml")
if err != nil {
    log.Fatal(err)
}

// Integrate with tokenizer
tokenizer := ingest.NewTokenizer(stopwords)
tokenizer.SetLexicon(lex)

// Now tokenization includes normalization:
// "Machine Learning models" → ["ml", "model"]
//   "machine" + "learning" → "ml" (via lexicon)
//   "models" stays as "models" (or normalized if in lexicon)
```

### 4. Query Expansion with C-Tokens

Add c-token relationships to lexicon (from corpus analysis or manual curation):

```go
// Add c-token relationships discovered from skip-gram analysis
stats := analyzer.Snapshot()
ctokens := stats.CTokenPairs(minSupport)

for _, ct := range ctokens {
    lex.AddCToken(ct.TokenA, lexicon.CToken{
        Token:   ct.TokenB,
        PMI:     ct.PMI,
        Support: ct.Support,
    })
}

// Query expansion
query := "transformer"
canonical := lex.Normalize(query)        // "transformer"
variants := lex.Variants(query)          // ["transformer", "transformers"]
ctokens := lex.GetCTokens(query)         // [{Token: "attention", PMI: 3.2, Support: 120}, ...]

// Use variants + c-tokens for comprehensive search
expandedQuery := append(variants, extractTokens(ctokens)...)
```

## Bootstrap Tool

### Synonym Candidate Discovery

The bootstrap tool uses **skip-gram c-token analysis** to suggest synonym groups:

**Algorithm**:
1. Analyze corpus with configurable skip-gram window (default: 5)
2. Calculate PMI for all token pairs within windows
3. Filter by minimum support and PMI thresholds
4. Build graph of high-PMI relationships
5. Find connected components (groups of mutually related tokens)
6. Output as synonym candidates sorted by PMI strength

**Configuration Flags**:
```
--synonym-limit          Maximum number of synonym candidates (default: 20)
--synonym-min-support    Minimum co-occurrence count (default: 3)
--synonym-min-pmi        Minimum PMI threshold (default: 1.5)
```

**Heuristics for Quality**:
- Higher PMI = stronger contextual relationship
- Higher support = more reliable statistics
- Connected components group tokens with shared context
- Manual review required to distinguish synonyms from collocations

### Full Bootstrap Command

```bash
./bootstrap \
  --input corpus.jsonl \
  --domain "ai-research" \
  --output configs/ai \
  --base-stoplist configs/stopwords-en.yaml \
  --stop-limit 25 \
  --synonym-limit 20 \
  --synonym-min-support 3 \
  --synonym-min-pmi 1.5 \
  --pair-limit 10 \
  --pair-min-support 3 \
  --pair-min-pmi 1.0 \
  --taxonomy-limit 150 \
  --iterations 2
```

Outputs:
- `stoplist.yaml` - Stopwords (high DF, low PMI)
- `synonyms.yaml` - Synonym candidates (high PMI c-tokens)
- `tokens.dict` - Multi-token phrases (high bigram + PMI)
- `taxonomies.yaml` - Category keywords (clustering)
- `bootstrap-report.json` - Statistics and metadata

## API Reference

### Lexicon

**Core Operations**:
```go
// Create new lexicon
lex := lexicon.New()

// Add synonym group
lex.AddSynonymGroup(canonical, variants)

// Normalization (variant → canonical)
normalized := lex.Normalize("gaming")  // "game"

// Expansion (canonical → all variants)
variants := lex.Variants("game")  // ["game", "games", "gaming", "gamer"]

// Check if token has synonyms
hasSyns := lex.HasSynonyms("gaming")  // true

// Add c-token relationship
lex.AddCToken("transformer", lexicon.CToken{
    Token:   "attention",
    PMI:     3.2,
    Support: 120,
})

// Get c-tokens for query expansion
ctokens := lex.GetCTokens("transformer")
// [{Token: "attention", PMI: 3.2, Support: 120}, ...]

// Load from YAML file
lex, err := lexicon.LoadFromYAML("synonyms.yaml")

// Get statistics
stats := lex.Stats()
// {SynonymGroups: 10, TotalVariants: 45, CTokenEntries: 8, TotalCTokens: 24}
```

### Analytics

**Skip-Gram Analysis**:
```go
// Create analyzer with custom window size
analyzer := analytics.NewAnalyzerWithWindow(5)

// Process documents
analyzer.Process(tokens, categories)

// Get statistics
stats := analyzer.Snapshot()

// Get c-token pairs (sorted by PMI descending)
ctokens := stats.CTokenPairs(minSupport)
// [{TokenA: "transformer", TokenB: "attention", PMI: 3.2, Support: 120}, ...]

// Get phrase candidates (bigram + PMI)
pairs := stats.TopPairs(limit, minPMI)
// [{A: "machine", B: "learning", PMI: 2.8, BigramFreq: 45, PhraseScore: 126}, ...]
```

### Tokenizer Integration

**3-Stage Pipeline**:
```go
tokenizer := ingest.NewTokenizer(stopwords)
tokenizer.SetLexicon(lex)

// Tokenization applies:
// 1. Clean: remove hyphens, normalize punctuation
// 2. Normalize: lexicon.Normalize(token) if lexicon set
// 3. Filter: remove stopwords

tokens := tokenizer.Tokenize("Machine Learning and AI research")
// With lexicon: ["ml", "ai", "research"]
// Without: ["machine", "learning", "ai", "research"]
```

## Examples

### Complete Integration Example

See `examples/lexicon-usage/main.go` for a full workflow demonstration:

```go
// 1. Analyze corpus → discover c-tokens
analyzer := analytics.NewAnalyzerWithWindow(5)
analyzer.Process(tokens, categories)
stats := analyzer.Snapshot()
ctokens := stats.CTokenPairs(minSupport)

// 2. Create lexicon with synonyms
lex := lexicon.New()
lex.AddSynonymGroup("ml", []string{"ml", "ML", "machine learning"})
lex.AddCToken("ml", lexicon.CToken{Token: "ai", PMI: 3.0, Support: 50})

// 3. Re-tokenize with normalization
tokenizer.SetLexicon(lex)
normalizedTokens := tokenizer.Tokenize(text)

// 4. Query expansion
query := "ml"
expanded := append(lex.Variants(query), extractCTokens(lex.GetCTokens(query))...)
// ["ml", "ML", "machine learning", "ai", ...]
```

### Performance Characteristics

**Skip-Gram Window Size**:
- Larger window = more c-token pairs, slower processing
- Smaller window = fewer relationships, faster but may miss distant associations
- Default (5) balances coverage and performance

**Memory Usage**:
- Skip-grams deduplicated per document (not per position)
- Memory ~ O(tokens² × window) worst case
- Typical: ~10MB for 1000 docs with 100 tokens each

**Statistical Reliability**:
- Minimum corpus size: 100+ documents for stable PMI
- Minimum support: 3+ co-occurrences for reliable associations
- PMI threshold: 1.0-2.0 for general relationships, 2.0+ for strong associations

## Advanced Topics

### Multi-Token Variants

The lexicon supports multi-token variants (e.g., `"machine learning"` → `"ml"`), but the tokenizer processes them as separate tokens by default.

**Current behavior**:
```go
lex.AddSynonymGroup("ml", []string{"ML", "machine learning"})
tokens := tokenizer.Tokenize("Machine Learning is cool")
// Result: ["machine", "learning", "cool"]
// "machine" and "learning" are separate tokens
```

**For phrase matching**, use `ingest.MultiTokenParser` in the pipeline:
```go
pipeline := ingest.NewPipeline(
    tokenizer,
    ingest.NewMultiTokenParser(multiTokenDictionary),
    ingest.NewTaxonomy(),
)
```

### C-Token Deduplication

C-tokens track **document-level co-occurrence**, not term frequency:
- Each unique pair counted once per document
- Prevents duplicate tokens from inflating counts
- Example: `"transformer transformer attention"` → count `(transformer, attention)` once

**Implementation**:
```go
// Position-based deduplication per document
skipGramSeen := make(map[pair]struct{})
for i := 0; i < len(tokens); i++ {
    for j := i + 1; j < len(tokens) && j < i+windowSize; j++ {
        p := newPair(tokens[i], tokens[j])
        skipGramSeen[p] = struct{}{}  // Dedup by pair, not position
    }
}
// Increment once per document
for p := range skipGramSeen {
    skipGramCounts[p]++
}
```

### Reverse Index Cleanup

When re-adding a synonym group, the lexicon cleans up old reverse index entries:

```go
lex.AddSynonymGroup("game", []string{"games", "gaming"})
// "games" → "game", "gaming" → "game"

lex.AddSynonymGroup("game", []string{"games", "gamer"})
// "gaming" → "gaming" (no longer maps to "game")
// "gamer" → "game" (new mapping)
```

### C-Token Deduplication by Strength

When adding the same c-token multiple times, the lexicon keeps the strongest relationship:

```go
lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.0, Support: 50})
lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.5, Support: 60})
// Keeps: PMI 2.5, Support 60 (higher PMI wins)

lex.AddCToken("transformer", CToken{Token: "bert", PMI: 2.0, Support: 100})
lex.AddCToken("transformer", CToken{Token: "bert", PMI: 2.0, Support: 80})
// Keeps: PMI 2.0, Support 100 (same PMI, higher support wins)
```

## Testing

Run lexicon tests:
```bash
go test ./pkg/korel/lexicon -v
```

Run analytics skip-gram tests:
```bash
go test ./pkg/korel/analytics -v
```

Run tokenizer lexicon integration tests:
```bash
go test ./pkg/korel/ingest -v -run TestTokenizerLexicon
```

## References

- **PMI (Pointwise Mutual Information)**: Measures association strength between tokens
  - Formula: `log(P(A,B) / (P(A) × P(B)))`
  - Higher PMI = stronger association
  - Used for both synonym candidates and phrase detection

- **Skip-grams**: Window-based co-occurrence (vs bigrams = adjacent only)
  - Captures contextual relationships beyond immediate adjacency
  - Configurable window size controls context distance

- **Connected Components**: Graph algorithm for grouping related tokens
  - Finds clusters of mutually related tokens
  - Used for synonym candidate discovery

## Future Enhancements

- [ ] LLM-assisted synonym validation (reduce false positives)
- [ ] Multi-token phrase normalization in tokenizer
- [ ] Automatic c-token bootstrapping from corpus
- [ ] Query expansion with PMI-weighted term boosting
- [ ] Cross-lingual synonym mappings
