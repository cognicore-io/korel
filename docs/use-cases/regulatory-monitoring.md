# Use Case: Regulatory Change Monitoring

## Problem

Compliance teams at financial institutions, pharma companies, and regulated manufacturers must track regulatory changes across hundreds of sources. Today this means:

- A team of analysts reading documents manually
- Keyword alerts that miss semantic variations ("digital asset" vs "cryptocurrency" vs "virtual currency")
- No systematic way to detect when new regulation topics emerge
- Audit findings when monitoring gaps are discovered after the fact

When a new regulation area appears (e.g., AI governance, ESG reporting, crypto asset regulation), the monitoring taxonomy doesn't cover it until someone manually notices — which can take months.

## Solution

Korel ingests regulatory documents and applies three capabilities that transform monitoring:

### 1. Bootstrap the Domain

Start with raw regulatory texts — no manual taxonomy needed:

```bash
# Download and prepare regulatory corpus (JSONL format)
# Each line: {"url": "...", "title": "...", "text": "...", "published_at": "..."}

# Bootstrap discovers domain structure automatically
go run ./cmd/bootstrap \
  -input data/regulations.jsonl \
  -domain finance-regulation \
  -output configs/finance-regulation

# Generated files:
#   stoplist.yaml     — domain-specific noise words
#   tokens.dict       — multi-token phrases ("anti-money laundering", "capital requirement")
#   taxonomies.yaml   — discovered categories (banking, securities, insurance, ...)
```

### 2. Ingest and Index

```bash
go run ./cmd/rss-indexer \
  -db ./data/regulations.db \
  -data data/regulations.jsonl \
  -stoplist configs/finance-regulation/stoplist.yaml \
  -dict configs/finance-regulation/tokens.dict \
  -taxonomy configs/finance-regulation/taxonomies.yaml
```

### 3. Detect Regulatory Drift

Run AutoTune periodically (weekly/monthly) on new documents. Taxonomy drift detection reveals:

- **Stale keywords** — Regulatory terms in the taxonomy that no longer appear in recent documents. These monitoring categories may need retirement or indicate a coverage gap in source selection.
- **Orphan tokens** — New terms appearing frequently across regulatory sources but not assigned to any monitoring category. These represent emerging regulation areas.

```go
result, _ := engine.AutoTune(ctx, newDocuments, &korel.AutoTuneOptions{
    BaseStopwords: currentStopwords,
})

for _, s := range result.TaxonomySuggestions {
    switch s.Type {
    case "low_coverage":
        // "Basel III" appears in taxonomy but 0 recent docs mention it
        // → Regulation may have been superseded, or source coverage is incomplete
        fmt.Printf("STALE: %q in category %q — review needed\n", s.Keyword, s.Category)
    case "orphan":
        // "digital operational resilience" appears in 15% of new docs
        // but isn't in any monitoring category
        // → DORA regulation is emerging, taxonomy needs updating
        fmt.Printf("NEW TOPIC: %q (suggest: %q) — add to monitoring\n", s.Keyword, s.Category)
    }
}
```

### 4. Explainable Retrieval for Auditors

When auditors ask "show me everything related to anti-money laundering," the results include score breakdowns:

```bash
go run ./cmd/chat-cli \
  -db ./data/regulations.db \
  -stoplist configs/finance-regulation/stoplist.yaml \
  -dict configs/finance-regulation/tokens.dict \
  -taxonomy configs/finance-regulation/taxonomies.yaml \
  -query "anti-money laundering"
```

```
--- Card 1: AML Directive Update ---
  Score Breakdown:
    pmi: 1.82      ← "anti-money laundering" strongly co-occurs with doc terms
    cats: 0.90     ← category match: banking, compliance
    recency: 0.75  ← published 3 days ago
    authority: 0.40 ← linked from 12 other documents
```

Every score component is auditable. Internal audit can verify *why* each document was returned and challenge the ranking.

## Benefits

- **Early warning on regulatory change** — Orphan tokens surface new regulation areas weeks or months before they'd be noticed manually.
- **Audit-ready retrieval** — Score breakdowns satisfy regulatory audit requirements (MiFID II, SOX, Basel). Every search result is explainable.
- **No hallucination risk** — Unlike LLM-based tools, Korel never fabricates regulatory citations. Every result links to a real document.
- **Reduced analyst burden** — AutoTune replaces manual taxonomy maintenance. Analysts review suggestions rather than building categories from scratch.
- **Multi-token precision** — "Anti-money laundering", "capital adequacy ratio", "central counterparty clearing" are treated as single concepts, not bags of words.

## Integration Pattern

```
Regulatory Sources (RSS, APIs, PDFs)
        │
        ▼
  Weekly Ingest (rss-indexer)
        │
        ▼
  Monthly AutoTune ──► Drift Report ──► Compliance Team Reviews
        │
        ▼
  Analyst Queries (chat-cli or API)
        │
        ▼
  Explainable Results with Audit Trail
```

Korel runs on a single server. No GPU, no cloud API, no per-query costs. Regulatory documents stay on-premise.
