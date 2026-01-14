# Symbolic Reasoning in Korel

## Overview

Korel combines **statistical methods (PMI)** with **symbolic reasoning (rule engines)** - a unique hybrid approach that provides both data-driven discovery and logical explainability.

## Why This Matters

Traditional RAG systems use opaque neural embeddings. Korel uses two transparent, complementary paradigms:

| Approach | Strength | Weakness | In Korel |
|----------|----------|----------|----------|
| Statistical (PMI) | Discovers patterns from data | Can't explain WHY relationships exist | ✅ Phase 1-5 |
| Symbolic (Rules) | Explains relationships logically | Can't discover NEW patterns | ✅ Phase 7 |

**Together:** Data discovers → Logic explains → User understands

## Implementation

### Pure Go Engine

```go
// pkg/korel/inference/inference.go - Interface
type Engine interface {
    LoadRules(rules string) error
    Query(relation, subject, object string) bool
    QueryAll(relation, subject string) []string
    FindPath(subject, object string) []Step
    Expand(tokens []string) []string  // ← Key for search integration
    Explain(relation, subject, object string) string
}
```

### Simple Implementation

```go
// pkg/korel/inference/simple/engine.go
// Minimal rule engine with:
// - Transitive closure (is_a relationships)
// - Proof chain generation
// - Query expansion
// - No external dependencies
```

### Rule Format

Simple, readable syntax:

```prolog
# configs/rules/ai.rules
is_a(bert, transformer)
is_a(transformer, neural-network)
used_for(transformer, nlp)
requires(transformer, attention-mechanism)
related_to(bert, masked-language-modeling)
```

## Integration with Search Pipeline

```
1. User Query: "bert"
   ↓
2. Parse → tokens: [bert]
   ↓
3. Symbolic Expansion → [bert, transformer, neural-network, nlp, attention-mechanism]
   ↓
4. Statistical Retrieval → Find docs with PMI scores
   ↓
5. Hybrid Scoring → α·PMI + θ·inference
   ↓
6. Explainable Card:
   • Statistical: bert ↔ transformer (PMI: 2.3)
   • Logical: bert is_a transformer → is_a neural-network
   • Proof chain shown to user
```

## Example: Query Expansion

**Without symbolic reasoning:**
```
Query: "bert"
Retrieves: Only documents explicitly mentioning "bert"
```

**With symbolic reasoning:**
```
Query: "bert"
Infers: bert is_a transformer, transformer is_a neural-network
Expands to: [bert, transformer, neural-network, attention-mechanism]
Retrieves: Documents about transformers, NNs, attention (even if they don't say "bert")
Result: More comprehensive + explained why each term was included
```

## Future: Self-Learning Rules

**Vision:** Use PMI statistics to suggest symbolic rules automatically:

```python
# If PMI(transformer, attention-mechanism) > 2.0 in 80% of docs:
# → Suggest rule: requires(transformer, attention-mechanism)

# If bert and transformer always co-occur:
# → Suggest rule: is_a(bert, transformer)

# Human validates → Rule added to knowledge base
```

This creates a **virtuous cycle**:
1. Statistics discover patterns
2. Humans codify as rules
3. Rules improve future searches
4. Better searches → better statistics → better rules

## Swappable Design

The `inference.Engine` interface allows upgrading:

**Current:** Simple Go engine (transitive closure, basic relations)

**Future options:**
- Full Prolog engine (golog, SWI-Prolog)
- Datalog variant
- Custom domain-specific logic
- Integration with existing knowledge graphs

## Testing

```bash
# Run symbolic reasoning tests
go test ./pkg/korel/inference/simple/... -v

# All tests pass:
# ✓ TestBasicFacts
# ✓ TestQueryAll  
# ✓ TestExpand
# ✓ TestLoadRules
# ✓ TestFindPath
# ✓ TestExplain
```

## Key Features

**Hybrid Architecture:** Korel integrates statistical co-occurrence analysis (PMI) with symbolic reasoning (rule engines):

- **Statistical layer** discovers patterns from corpus data
- **Symbolic layer** encodes domain knowledge and relationships
- **Integration layer** combines both for query expansion and scoring

**Technical approach:**
- Pure Go implementation (no external dependencies)
- Swappable inference engine interface
- Transitive closure for taxonomic reasoning
- Explainable proof chains for every inference
- Transparent hybrid scoring formula

## References

- `pkg/korel/inference/` - Interface definition
- `pkg/korel/inference/simple/` - Go implementation  
- `configs/rules/` - Example rule files
- `README.md` - Full architecture overview
