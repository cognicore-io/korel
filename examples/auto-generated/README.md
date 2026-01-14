# Auto-Generated Examples

**IMPORTANT:** These files are **examples of what Korel should automatically generate** from corpus analysis. They are NOT meant to be manually maintained.

## Purpose

These files demonstrate Korel's self-learning capabilities:

1. **stoplist.yaml** - Discovered from corpus statistics:
   - High document frequency (appears in 80%+ of documents)
   - Low PMI_max (no strong associations with specific terms)
   - High category entropy (uniformly distributed across categories)

2. **tokens.dict** - Multi-token phrases discovered from:
   - N-gram frequency analysis
   - Boundary detection (words that frequently appear together)
   - Synonym detection (terms that co-occur with same neighbors)

3. **taxonomies.yaml** - Categories learned from:
   - Token clustering (PMI-based similarity)
   - Domain patterns (what words co-occur with known categories)
   - Entity recognition (proper nouns, tickers, locations)

4. **rules/*.rules** - Symbolic rules suggested from:
   - High PMI → `related_to(X, Y)` candidates
   - Co-occurrence >80% → `requires(X, Y)` candidates
   - Hierarchical clustering → `is_a(X, Y)` candidates

## How to Use

### For Testing
```go
// Load as test fixtures
stoplist, _ := config.LoadStoplist("examples/auto-generated/hn/stoplist.yaml")
dict, _ := config.LoadDict("examples/auto-generated/hn/tokens.dict")
```

### As Target Output
When implementing auto-generation:
```go
// After processing 1000 documents from HN:
generatedStoplist := analyzer.DetectStopwords(corpus)
// Compare with examples/auto-generated/hn/stoplist.yaml
// Did we discover the same patterns?
```

### Manual Bootstrap (Optional)
You CAN manually seed the system with domain knowledge:
```bash
# Copy examples to testdata/ as starting point
cp examples/auto-generated/hn/* testdata/hn/
# System will refine/expand based on actual corpus
```

But the goal is: **Start with minimal seeds, let statistics discover the rest.**

## File Origin

These files were created manually to demonstrate:
- What successful auto-generation looks like
- Domain-specific terminology (HN: tech, arXiv: academic, News: energy)
- Complexity level the system should achieve

They serve as:
- ✅ Test fixtures for unit tests
- ✅ Validation targets for auto-generation algorithms
- ✅ Documentation of expected output format
- ❌ NOT production configuration (should be generated)

## Future: Full Auto-Generation

Planned pipeline:
```
Raw Corpus → Tokenization → Statistics Collection →
  ├─ Stopword Detection (DF%, PMI, entropy) → stoplist.yaml
  ├─ Multi-token Discovery (n-gram freq) → tokens.dict
  ├─ Category Clustering (PMI similarity) → taxonomies.yaml
  └─ Rule Suggestion (PMI patterns) → rules/*.rules
                                          ↓
                                   Human validates
                                          ↓
                                   Add to knowledge base
```

## Directories

- `hn/` - Hacker News domain (tech, startups, programming)
- `arxiv/` - Academic papers (AI, ML, research)
- `news/` - Energy/renewables news (policy, finance, solar)

Each demonstrates different vocabulary, multi-token patterns, and domain structure.
