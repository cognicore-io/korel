# Korel Analytics

This package exposes lightweight analytics helpers used to **bootstrap** and
**monitor** new corpora. It ingests token/category streams, aggregates corpus
statistics (document frequency, category entropy, etc.), and exposes them in the
same format consumed by the autotune modules.

## Dual Tracking Architecture

The analyzer maintains **two types of pair statistics** for different purposes:

### 1. Bigram Tracking (Adjacent Pairs)
**Purpose:** Phrase discovery - identifies fixed collocations

Counts only consecutive token pairs in sequence:
```go
text := "deep learning models use deep neural networks"
// Bigrams (preserving order):
// (deep,learning), (learning,models), (models,use),
// (use,deep), (deep,neural), (neural,networks)
```

**Use case:** Finding multi-token terms like "machine learning", "neural networks"

### 2. Document-level Co-occurrence (All Pairs)
**Purpose:** Semantic filtering - measures conceptual relatedness via PMI

Counts all unique token pairs within each document:
```go
// Same text → all combinations of unique tokens (alphabetically sorted)
// (deep,learning), (deep,models), (deep,neural), (deep,networks), (deep,use),
// (learning,models), (learning,neural), (learning,networks), (learning,use), ...
// 15 pairs total
```

**Use case:** Calculating PMI to filter out stopword adjacencies ("can be", "has been")

**Summary of tracking responsibilities:**
- **Bigram tracker**: surfaces dictionary-worthy phrases; sensitive to order, so `"deep learning"` ≠ `"learning deep"`.
- **Document co-occurrence tracker**: surfaces semantically-related concepts used anywhere in the same document to feed PMI-driven stopword detection, taxonomy seeding, and explainability statistics.

### Combined Scoring for Phrase Discovery

Both signals are combined to rank phrase candidates:

```go
phraseScore = bigramFrequency × documentPMI
```

This filters out:
- **Stopword adjacencies:** High bigram freq, low PMI (e.g., "can be")
- **Non-collocations:** Low bigram freq, high PMI (e.g., "learning theory" - related but not a fixed phrase)

**Concrete dual tracking example (taken from an `arxiv-iterative` bootstrap run):**

```
Bigrams:       (deep,learning) appears adjacent 450 times
Document PMI:  "deep" and "learning" co-occur in 180 documents with PMI 0.59
Combined:      phraseScore = 450 × 0.59 = 265.5  → qualifies for tokens.dict
Stopwords:     "can be" has bigramFrequency=90 but PMI=0.03 → score=2.7 → stopword queue
```

### Tracking Method Use Cases

- **Bigram tracking**
  - Emit stable phrases for `tokens.dict`
  - Measure degradation when a stopword candidate accidentally removes parts of a phrase
- **Document-level PMI tracking**
  - Rank stopword candidates: high document frequency + low PMI
  - Seed taxonomy clusters by looking at which concepts co-occur with industry terms
  - Provide hybrid ranking features (document support counts) to the scoring engine

## Usage

Typical flow for a brand-new dataset:

1. Run the bootstrap CLI (`cmd/bootstrap`) with iterative refinement enabled
2. Review the generated reports: high-DF/low-PMI tokens (stopword candidates),
   bigram-PMI combined scores (phrase candidates), category coverage
3. Commit the suggestions into `configs/` (stoplist, taxonomy, dictionary)
4. Ingest the corpus normally, then rely on the autotune+maintenance loop for
   ongoing refinement

Because the analytics layer reuses the same data structures as the autotuners, it
slots directly into the broader bootstrap workflow.

## Example

```go
analyzer := analytics.NewAnalyzer()

for _, doc := range documents {
    tokens := tokenizer.Tokenize(doc.Text)
    analyzer.Process(tokens, doc.Categories)
}

stats := analyzer.Snapshot()

// Get phrase candidates (sorted by bigramFreq × PMI)
pairs := stats.TopPairs(50, 0.5) // minPMI=0.5
for _, pair := range pairs {
    fmt.Printf("%s %s: freq=%d, PMI=%.2f, score=%.1f\n",
        pair.A, pair.B, pair.BigramFreq, pair.PMI, pair.PhraseScore)
}
// Output:
// machine learning: freq=98, PMI=0.59, score=58.0
// can be: freq=90, PMI=0.03, score=2.7 (filtered next iteration)
// deep learning: freq=450, PMI=0.59, score=265.5
```

Expected terminal output for that snippet:

```
machine learning: freq=98, PMI=0.59, score=58.0
can be: freq=90, PMI=0.03, score=2.7
deep learning: freq=450, PMI=0.59, score=265.5
```
