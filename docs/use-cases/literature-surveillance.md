# Use Case: Scientific Literature Surveillance

## Problem

Pharma companies, research institutions, and clinical teams need to systematically monitor published literature. This is both a regulatory requirement (FDA post-market surveillance) and a research need (systematic reviews). Current approaches have serious gaps:

- **Keyword search misses semantic connections** — Searching "headache" won't find papers discussing "cephalgia" or "migraine-type adverse event" unless those exact terms are in the query.
- **LLMs fabricate citations** — Researchers have submitted papers citing studies that don't exist, generated confidently by language models.
- **Results aren't reproducible** — Vector embeddings change between model versions. A search run today and repeated in 6 months for regulatory submission may return different results.
- **New terminology goes undetected** — When a new adverse event term appears in literature, keyword-based monitoring doesn't catch it until someone manually adds it to the search strategy.

## Solution

### 1. Ingest Research Corpus

Korel works with any JSONL corpus. For academic papers, the arXiv downloader is included:

```bash
# Download AI research papers
go run ./cmd/download-arxiv cs.AI 500

# Or prepare custom corpus from PubMed/clinical sources
# Format: {"url": "...", "title": "...", "text": "...", "published_at": "..."}
```

Bootstrap domain-specific configuration:

```bash
go run ./cmd/bootstrap \
  -input data/pubmed-cardiology.jsonl \
  -domain cardiology \
  -output configs/cardiology
```

This discovers:
- **Stopwords** — Generic medical terms that appear everywhere ("patient", "study", "results")
- **Multi-token phrases** — "myocardial infarction", "left ventricular ejection fraction", "randomized controlled trial"
- **Category structure** — Clusters around pathology, treatment, diagnostics, outcomes

### 2. Index and Search

```bash
go run ./cmd/rss-indexer \
  -db ./data/cardiology.db \
  -data data/pubmed-cardiology.jsonl \
  -stoplist configs/cardiology/stoplist.yaml \
  -dict configs/cardiology/tokens.dict \
  -taxonomy configs/cardiology/taxonomies.yaml \
  -rules configs/rules/cardiology.rules
```

### 3. Multi-Hop Query Expansion

Symbolic rules enable transitive search. Define domain knowledge:

```prolog
# configs/rules/cardiology.rules
is_a(aspirin, nsaid)
is_a(ibuprofen, nsaid)
is_a(nsaid, anti-inflammatory)
used_for(statin, hyperlipidemia)
related_to(myocardial-infarction, troponin)
related_to(heart-failure, bnp)
```

Now querying "NSAID cardiac risk" expands through the inference chain:

```
Query: "NSAID cardiac risk"
    ↓ symbolic expansion
Expanded: [nsaid, aspirin, ibuprofen, anti-inflammatory, cardiac, risk]
    ↓ PMI discovery
Also finds: documents about "cox-2 inhibitor cardiovascular events"
    ↓ proof chain
Explanation:
  nsaid → aspirin (is_a, depth 1, confidence 0.70)
  nsaid → anti-inflammatory (is_a, depth 1, confidence 0.70)
  nsaid ↔ cardiovascular (PMI 1.8, corpus co-occurrence)
```

### 4. Emerging Signal Detection

Run AutoTune on newly ingested papers monthly:

```go
result, _ := engine.AutoTune(ctx, newAbstracts, opts)

// Orphan tokens = new terminology appearing in literature
// that the monitoring taxonomy doesn't cover
for _, s := range result.TaxonomySuggestions {
    if s.Type == "orphan" && s.Confidence > 0.7 {
        // "GLP-1 receptor agonist" appearing in 8% of new papers
        // but not in cardiology taxonomy
        // → New drug class relevant to cardiac outcomes
        log.Printf("NEW TERM: %q (%d papers, suggest category: %q)",
            s.Keyword, s.MissedDocs, s.Category)
    }
}

// High-PMI rules = newly discovered term relationships
for _, r := range result.RuleSuggestions {
    if r.Confidence > 0.6 {
        // related_to(sglt2-inhibitor, heart-failure) discovered from co-occurrence
        log.Printf("NEW RELATIONSHIP: %s(%s, %s) confidence=%.2f",
            r.Relation, r.Subject, r.Object, r.Confidence)
    }
}
```

### 5. Reproducible Search Strategy

For systematic reviews and regulatory submissions, deterministic results are critical:

```bash
# Run search today
go run ./cmd/chat-cli \
  -db ./data/cardiology.db \
  -stoplist configs/cardiology/stoplist.yaml \
  -dict configs/cardiology/tokens.dict \
  -taxonomy configs/cardiology/taxonomies.yaml \
  -query "statin adverse events elderly" \
  -topk 20

# Run exact same search 6 months later for FDA submission
# → identical results (same corpus, same query, same output)
```

No embedding model version changes. No temperature randomness. The search strategy is fully specified by the query, the corpus, and the configuration files — all version-controlled.

## Benefits

- **No fabricated citations** — Every result is a real document with a real URL. Eliminates the risk of submitting phantom references.
- **Reproducible for regulatory submission** — Same corpus + same query = same results, today and in 6 months. Satisfies FDA validation requirements.
- **Catches emerging terminology** — Orphan detection surfaces new drug names, adverse event terms, and treatment approaches as they appear in literature — before they're in the monitoring taxonomy.
- **Relationship discovery** — PMI finds that "troponin" and "myocardial infarction" co-occur strongly, even without manual rule definition. AutoTune converts these into symbolic rules for future query expansion.
- **Explainable inclusion/exclusion** — For systematic reviews, every included and excluded document has a transparent score showing why it was ranked where it was.
- **No GPU costs** — Runs on a workstation. A department of 50 researchers can share one Korel instance.

## Example: Adverse Event Monitoring

A pharmacovigilance team monitors literature for a cardiovascular drug:

```
Monthly Drift Report:

STALE — Keywords absent from recent literature:
  "digitalis"     category=treatment    0 papers — term rarely used now
  "fibrinolytic"  category=treatment    2 papers (1% coverage)

ORPHAN — New terms appearing in literature:
  "pcsk9"         suggest=treatment     28 papers (14% of corpus)
  "sglt2"         suggest=treatment     19 papers (10% of corpus)
  "entresto"      suggest=treatment     12 papers (6% of corpus)

ACTION: PCSK9 inhibitors and SGLT2 inhibitors are new drug classes
appearing in cardiac literature. Add to monitoring taxonomy to avoid
missing relevant safety signals.
```

This catches the emergence of new drug classes in the cardiac safety literature — before adverse events are reported through traditional pharmacovigilance channels.
