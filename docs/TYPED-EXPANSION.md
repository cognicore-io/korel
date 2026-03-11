# Typed Expansion

Typed expansion classifies PMI co-occurrence edges into semantic
relationship types — **synonym**, **broader**, **narrower** — enabling
directional query expansion that is more precise than flat PMI
association.

## Problem

Korel's PMI expansion finds statistically related terms but cannot
distinguish *why* they are related. "kubernetes" and "container" have
high PMI, but the expansion cannot tell you that "container" is a
broader concept while "pod" is a narrower one. This makes expansion
noisy: every direction gets equal weight.

## Solution

A distributional classifier (`pkg/korel/reltype`) infers relationship
types from statistics Korel already computes:

| Relationship | Signal | Example |
|-------------|--------|---------|
| **same_as** | High neighbor overlap + low co-occurrence (writers pick one form) | k8s / kubernetes |
| **broader** | A's contexts are a subset of B's (distributional inclusion) | pod → kubernetes |
| **narrower** | B's contexts are a subset of A's | kubernetes → pod |
| **related_to** | Default — symmetric statistical association | kubernetes / docker |

### Synonym Detection

True synonyms share nearly identical distributional contexts but
rarely appear together in the same document. The classifier measures:

1. **Neighbor overlap** — Jaccard similarity of top-K PMI neighbors
2. **Co-occurrence ratio** — `P(A,B) / min(P(A), P(B))`

High overlap + low co-occurrence = synonym.

### Hypernymy Detection (Broader/Narrower)

Uses the **distributional inclusion hypothesis** (Weeds & Weir 2003):
if every context where "pod" appears also contains "kubernetes" (but
not vice versa), then "kubernetes" is a broader concept. Measured via
Weeds precision:

```
WP(A→B) = |neighbors(A) ∩ neighbors(B)| / |neighbors(A)|
```

When WP(A→B) >> WP(B→A), B is broader than A.

## Architecture

```
Corpus → PMI computation → related_to edges (existing)
                                    ↓
                          reltype.Classifier (new, optional)
                                    ↓
                      same_as / broader / narrower edges
                                    ↓
                          Prolog inference engine
                                    ↓
                     Typed expansion rules (new)
                                    ↓
                       Search with ExpandMode
```

### Packages

- **`pkg/korel/reltype`** — Distributional relationship classifier.
  Stateless, operates on neighbor lists and document frequencies.
- **`pkg/korel/inference/prolog/rules.go`** — New Prolog rules for
  typed expansion (`expand_synonym`, `expand_broader`,
  `expand_narrower`, `expand_typed`).
- **`pkg/korel/graph.go`** — `classifyEdges()` runs the classifier
  over PMI edges during `BuildGraph`.
- **`pkg/korel/search.go`** — `ExpandMode` field on `SearchRequest`
  selects which expansion direction to use.

## Usage

### Enabling Typed Expansion

Pass a `reltype.Config` when creating a Korel instance:

```go
import "github.com/cognicore/korel/pkg/korel/reltype"

cfg := reltype.DefaultConfig()
engine := korel.New(korel.Options{
    // ... existing options ...
    TypedExpansion: &cfg, // enables the classifier
})
```

When `TypedExpansion` is nil (default), behavior is unchanged — all
PMI edges remain untyped `related_to`.

### Search with ExpandMode

```go
resp, err := engine.Search(ctx, korel.SearchRequest{
    Query:      "kubernetes security",
    TopK:       10,
    ExpandMode: "synonyms", // only synonym/same_as edges
})
```

Available modes:

| Mode | Prolog Rule | Effect |
|------|-------------|--------|
| `""` or `"all"` | `expand_token` | Original behavior (all directions) |
| `"synonyms"` | `expand_synonym` | Only synonym/same_as edges |
| `"broader"` | `expand_broader` | Only broader (more general) concepts |
| `"narrower"` | `expand_narrower` | Only narrower (more specific) concepts |
| `"typed"` | `expand_typed` | All typed edges + original expansion |

### Classifier Configuration

```go
cfg := reltype.Config{
    MinNeighborOverlap:   0.3,  // Jaccard threshold for synonym candidates
    MaxCooccurrenceRatio: 0.5,  // max co-occurrence ratio for synonyms
    InclusionThreshold:   0.7,  // Weeds precision threshold for hypernymy
    MinConfidence:        0.5,  // minimum confidence for any typed classification
    TopK:                 20,   // number of neighbors to compare
}
```

## Prolog Rules

The typed expansion adds these rules to the inference engine:

```prolog
% Synonym expansion (symmetric)
expand_synonym(T, X) :- same_as(T, X).
expand_synonym(T, X) :- same_as(X, T).
expand_synonym(T, X) :- synonym(T, X).
expand_synonym(T, X) :- synonym(X, T).
expand_synonym(T, X) :- equivalent(T, X).

% Upward expansion (transitive)
expand_broader(T, X) :- broader(T, X).
expand_broader(T, X) :- broader(T, Z), broader(Z, X), T \= X.

% Downward expansion (transitive)
expand_narrower(T, X) :- narrower(T, X).
expand_narrower(T, X) :- narrower(T, Z), narrower(Z, X), T \= X.

% Combined typed expansion
expand_typed(T, X) :- expand_synonym(T, X).
expand_typed(T, X) :- expand_broader(T, X).
expand_typed(T, X) :- expand_narrower(T, X).
expand_typed(T, X) :- expand_token(T, X).
```

## Edge Storage

Typed edges are stored in the existing `edges` table with
`source="reltype"`. They coexist with the original `related_to` edges
(source="pmi"):

```
subject      | relation    | object                  | weight | source
-------------|-------------|-------------------------|--------|--------
k8s          | related_to  | kubernetes              | 0.92   | pmi
k8s          | same_as     | kubernetes              | 0.87   | reltype
pod          | related_to  | kubernetes              | 0.85   | pmi
pod          | broader     | kubernetes              | 0.78   | reltype
kubernetes   | narrower    | pod                     | 0.78   | reltype
```

## Testing

```bash
# Unit tests for the classifier
go test ./pkg/korel/reltype/ -v

# Prolog typed expansion rules
go test ./pkg/korel/inference/prolog/ -run TestPrologTypedExpansion -v
```

## Design Decisions

1. **Optional by default** — TypedExpansion=nil preserves existing
   behavior. No breaking changes.

2. **Typed edges are additive** — Original `related_to` edges stay.
   Typed edges are stored separately with `source="reltype"` and
   cleared/rebuilt independently.

3. **Classification runs during BuildGraph** — One-time cost at index
   time, not query time. Typed edges persist in SQLite.

4. **Prolog does the reasoning** — The classifier only proposes types.
   Transitive chains (pod → kubernetes → container-orchestration) are
   handled by Prolog rules, keeping the classifier simple.

5. **Co-occurrence estimation** — Since Korel stores PMI scores rather
   than raw co-occurrence counts in the edges table, the classifier
   estimates raw counts by inverting the PMI formula. This is
   approximate but sufficient for the co-occurrence ratio signal.

## Lessons from TaxMind (German Tax Law)

Testing against TaxMind's 7,970-document legal corpus revealed
important limitations of automatic classification:

### What doesn't work: Synonym auto-detection in legal corpora

The known synonym pairs (Abschreibung/AfA, Ehepartner/Ehegatte,
Homeoffice/Arbeitszimmer) cannot be detected distributional because:

1. **Colloquial terms barely exist in the corpus.** `afa` has df=0,
   `homeoffice` df=0, `ehepartner` df=2. Legal texts use formal
   language exclusively. You can't discover synonyms for terms that
   don't appear.

2. **Low neighbor overlap.** Even conceptually related pairs like
   Freibetrag/Grundfreibetrag share only 6/20 neighbors (Jaccard
   0.18). Legal paragraphs are self-contained units with specialized
   vocabulary — distributional signals are weak.

3. **The distributional inclusion hypothesis breaks down.** Even
   Verschmelzung/Umwandlung (clearly narrower/broader) share only
   1/20 neighbors. Legal structure (independent sections) defeats
   the assumption that contexts overlap.

### What works: Curated typed edges from config

The right approach for legal domains is **manual curation in config**:

```yaml
# synonyms.yaml
edges:
  - [same_as, abschreibung, afa]
  - [same_as, ehepartner, ehegatte]
  - [broader, lohnsteuer, einkommensteuer]
  - [broader, verschmelzung, umwandlung]
```

This is stored in `data/korel-config/synonyms.yaml` and loaded at
startup. Benefits:

- **Maintainable** — tax experts edit YAML, not Go code
- **Typed** — `same_as` vs `broader` vs `related_to` enables
  directional expansion via ExpandMode
- **No recompile** — add new synonyms without touching source code
- **Version controlled** — changes tracked alongside the corpus

### When auto-detection IS useful

The distributional classifier works best with:

- **Narrative corpora** (news, papers, blogs) where terms appear in
  flowing text with rich context
- **Large corpora** (10K+ documents) with sufficient term frequency
- **Terms that both appear in the corpus** — can't classify what's
  not there

For legal, medical, or other domain-specific corpora with formal
language, manual curation + typed edges is the recommended path.

## References

- Weeds, J., & Weir, D. (2003). A general framework for distributional
  generality. *EMNLP*.
- Kotlerman, L., et al. (2010). Directional distributional similarity
  for lexical inference. *Natural Language Engineering*.
- Lenci, A., & Benotto, G. (2012). Identifying hypernyms in
  distributional semantic spaces. *SEM*.
