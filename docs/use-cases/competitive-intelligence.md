# Use Case: Competitive Intelligence & Content Strategy

## Problem

Media companies, B2B publishers, and strategy teams monitor competitors and industry trends across hundreds of sources. Current approaches have blind spots:

- **Manual monitoring doesn't scale** — Analysts can track 10-20 sources. Industry-wide monitoring requires hundreds.
- **Keyword alerts are brittle** — Setting up alerts for "artificial intelligence" misses articles about "machine learning", "neural networks", or "foundation models" unless every variant is manually listed.
- **Trend detection is reactive** — By the time a topic appears in trend reports, competitors have been writing about it for months.
- **No systematic gap analysis** — "What are competitors covering that we're not?" is answered by gut feeling, not data.

## Solution

### 1. Build Competitive Corpus

Ingest articles from your own publication and competitor sources:

```bash
# Download tech news (or substitute your industry's sources)
go run ./cmd/download-hn 500

# Prepare competitor content as JSONL
# Each line: {"url": "...", "title": "...", "text": "...", "published_at": "...", "outlet": "competitor-name"}

# Bootstrap discovers topic structure across all sources
go run ./cmd/bootstrap \
  -input data/industry-corpus.jsonl \
  -domain tech-media \
  -output configs/tech-media
```

### 2. Index and Categorize

```bash
go run ./cmd/rss-indexer \
  -db ./data/industry.db \
  -data data/industry-corpus.jsonl \
  -stoplist configs/tech-media/stoplist.yaml \
  -dict configs/tech-media/tokens.dict \
  -taxonomy configs/tech-media/taxonomies.yaml
```

### 3. Discover Topic Relationships

PMI reveals which topics co-occur across the industry — showing narrative clusters:

```bash
go run ./cmd/chat-cli \
  -db ./data/industry.db \
  -stoplist configs/tech-media/stoplist.yaml \
  -dict configs/tech-media/tokens.dict \
  -taxonomy configs/tech-media/taxonomies.yaml \
  -query "ai regulation"
```

```
Explain:
  Query tokens: [ai, regulation]
  Expanded: [ai, regulation, governance, compliance, eu-ai-act, safety]
  Top pairs:
    ai ↔ regulation (PMI: 2.45)
    ai ↔ governance (PMI: 1.90)
    regulation ↔ eu-ai-act (PMI: 2.80)
    regulation ↔ compliance (PMI: 1.75)
    ai ↔ safety (PMI: 1.60)
```

These co-occurrence clusters show the current industry narrative: AI regulation discussions are linked to EU AI Act, governance frameworks, and safety standards. This isn't configured — it's discovered from the corpus.

### 4. Content Gap Analysis

The core value: compare your publication's taxonomy against the full industry corpus.

```go
// Index industry-wide corpus (competitors + your content)
// Set taxonomy to YOUR editorial categories
// Run AutoTune → drift report shows gaps

result, _ := engine.AutoTune(ctx, industryTexts, opts)

for _, s := range result.TaxonomySuggestions {
    if s.Type == "orphan" {
        // Topics appearing across industry sources that
        // YOUR editorial taxonomy doesn't cover
        fmt.Printf("COVERAGE GAP: %q — %d articles industry-wide, "+
            "not in your taxonomy\n", s.Keyword, s.MissedDocs)
    }
}
```

Example output:

```
COVERAGE GAP REPORT — vs. Industry Corpus

Topics competitors cover that you don't:
  "sovereign ai"          42 articles (8% of corpus) — no coverage
  "small language model"  38 articles (7% of corpus) — no coverage
  "ai agent framework"    31 articles (6% of corpus) — no coverage
  "synthetic data"        28 articles (5% of corpus) — no coverage

Your stale topics (in taxonomy, declining in industry):
  "blockchain"            3 articles (0.6%) — industry has moved on
  "metaverse"             1 article  (0.2%) — effectively dead topic
  "nft"                   0 articles        — completely absent
```

This answers "what should we be writing about?" with data, not intuition.

### 5. Trend Velocity

Track orphan tokens over time to detect acceleration:

```go
// Run monthly, compare orphan lists
// Tokens that appear as orphans for the first time = emerging topics
// Tokens that grow in doc count month-over-month = accelerating topics

// January: "sovereign ai" — 12 articles (2%)
// February: "sovereign ai" — 28 articles (5%)  ← accelerating
// March: "sovereign ai" — 42 articles (8%)     ← established trend
```

By the time "sovereign AI" appears in analyst reports, you've been covering it for two months.

### 6. Source Analysis via PMI Neighbors

Discover which sources drive specific topics:

```bash
go run ./cmd/chat-cli \
  -db ./data/industry.db \
  -stoplist configs/tech-media/stoplist.yaml \
  -dict configs/tech-media/tokens.dict \
  -taxonomy configs/tech-media/taxonomies.yaml \
  -query "sovereign ai" \
  -topk 10
```

Results grouped by source outlet show which competitors lead on which topics — and which topics have no dominant voice (opportunity).

## Benefits

- **Data-driven editorial planning** — Content gaps are identified by comparing your taxonomy against the industry corpus. No more guessing what to write about.
- **Early trend detection** — Orphan tokens surface emerging topics before they appear in industry reports. First-mover advantage on coverage.
- **Competitor blind spots** — Run the same analysis with competitor taxonomies. Topics that are orphans in THEIR coverage but not yours = your competitive advantage.
- **No per-query API cost** — Monitor hundreds of sources continuously. Index once per week, query unlimited. Unlike LLM-based tools where every analyst query costs money.
- **Relationship mapping** — PMI discovers which topics cluster together in industry narrative. "AI regulation" → "EU AI Act" + "governance" + "safety" shows the conversation structure.
- **Reproducible trend tracking** — Same corpus + same date range = same results. Month-over-month comparisons are meaningful because the analysis is deterministic.

## Integration Pattern

```
Industry Sources                      Editorial Team
  │ Competitor RSS                       │
  │ Industry newsletters    ┌─────────┐  │
  │ Press releases     ──►  │  Korel  │──┤ Weekly gap report
  │ Your own archive        │         │  │ Trend velocity dashboard
  │ Social/forum exports    └─────────┘  │ Topic relationship maps
  └──────────────────────►               │
         │                               │
   Weekly ingest                  "What should we
   Monthly drift report            cover next?"
```

## Practical Example: Tech Media Company

A B2B tech publisher monitors 200 sources to plan their editorial calendar:

**Week 1:** Ingest 2,000 new articles from industry RSS feeds.

**Week 2:** Run AutoTune. Drift report shows:
- "agentic AI" appears as orphan (new topic, 6% of corpus)
- "prompt engineering" declining (was 12%, now 4%)
- "RAG" still strong (15% of corpus, in taxonomy)

**Week 3:** Editorial meeting reviews drift report. Decision:
- Commission 3 articles on agentic AI (emerging topic)
- Deprioritize prompt engineering content (declining interest)
- Continue RAG coverage (sustained interest)

**Result:** Content strategy driven by corpus data instead of editorial intuition. Publication covers emerging topics before competitors react to the same signals manually.
