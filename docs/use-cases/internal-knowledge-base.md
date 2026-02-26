# Use Case: Internal Knowledge Base Search

## Problem

Growing engineering organizations accumulate thousands of internal documents — Confluence pages, design docs, runbooks, post-mortems, RFCs. Finding information becomes the bottleneck:

- **Search is broken** — Full-text search returns too many results for common terms and zero results for specific questions. Engineers give up and ask on Slack instead.
- **Tribal knowledge** — Critical information exists only in the heads of senior engineers. When they leave, the knowledge goes with them.
- **Stale documentation** — Nobody knows which docs are outdated. New employees follow obsolete runbooks and break things.
- **Enterprise search is expensive** — Tools like Glean, Guru, or Coveo charge $20-40/user/month. For a 500-person engineering org, that's $120K-240K/year.
- **LLM-based search has risks** — Sending internal documents to OpenAI's API creates IP exposure. Self-hosted LLMs require GPU infrastructure.

## Solution

### 1. Export and Ingest Documents

Convert internal docs to JSONL (one doc per line):

```bash
# Example: export from Confluence, Google Docs, or markdown files
# Format: {"url": "...", "title": "...", "text": "...", "published_at": "...", "outlet": "confluence"}

# Bootstrap discovers your company's terminology
go run ./cmd/bootstrap \
  -input data/internal-docs.jsonl \
  -domain engineering \
  -output configs/internal
```

The bootstrap command discovers without any manual configuration:
- **Stopwords** — Generic terms that appear in every document ("team", "please", "update", "meeting")
- **Multi-token phrases** — Internal jargon ("blue sky review", "tiger team", "incident commander", "design review board")
- **Categories** — Topic clusters (infrastructure, frontend, backend, data-platform, security, ...)

### 2. Index the Corpus

```bash
go run ./cmd/rss-indexer \
  -db ./data/internal.db \
  -data data/internal-docs.jsonl \
  -stoplist configs/internal/stoplist.yaml \
  -dict configs/internal/tokens.dict \
  -taxonomy configs/internal/taxonomies.yaml
```

### 3. Search with Context

```bash
go run ./cmd/chat-cli \
  -db ./data/internal.db \
  -stoplist configs/internal/stoplist.yaml \
  -dict configs/internal/tokens.dict \
  -taxonomy configs/internal/taxonomies.yaml \
  -query "database migration rollback procedure"
```

```
--- Card 1: Runbook: PostgreSQL Migration Rollback ---
  • Step-by-step rollback procedure for failed schema migrations
  • Covers both single-node and replicated setups

Sources:
  - https://confluence.internal/runbooks/pg-migration-rollback (2025-09-15)

Score Breakdown:
  pmi: 2.10      ← "database migration" ↔ "rollback" strongly co-occur
  cats: 0.80     ← category match: infrastructure, database
  recency: 0.60  ← 2 months old
  authority: 0.55 ← linked from 8 other documents

Explain:
  Query tokens: [database migration, rollback, procedure]
  Expanded: [database migration, rollback, procedure, schema, postgres]
  Top pairs:
    database migration ↔ rollback (PMI: 2.10)
    rollback ↔ schema (PMI: 1.65)
```

PMI automatically discovers that "database migration" and "rollback" co-occur — no synonym dictionary needed. Multi-token recognition treats "database migration" as a single concept.

### 4. Content Gap Analysis

Run AutoTune monthly on the full document corpus:

```go
result, _ := engine.AutoTune(ctx, allDocTexts, opts)

for _, s := range result.TaxonomySuggestions {
    switch s.Type {
    case "low_coverage":
        // "docker swarm" is in the taxonomy but 0 recent docs mention it
        // → team migrated to Kubernetes, docs are stale
        fmt.Printf("STALE TOPIC: %q in %q — docs may be outdated\n",
            s.Keyword, s.Category)
    case "orphan":
        // "opentelemetry" appears in 18% of recent docs but
        // isn't in any taxonomy category
        // → new technology adopted without documentation coverage
        fmt.Printf("UNDOCUMENTED: %q (%d docs) — needs documentation\n",
            s.Keyword, s.MissedDocs)
    }
}
```

Example monthly report:

```
Content Health Report — January 2026

STALE TOPICS (in taxonomy but absent from recent docs):
  "docker swarm"     infra       0 docs — migrated to k8s, archive these
  "jenkins"          ci-cd       2 docs — replaced by GitHub Actions
  "angular"          frontend    1 doc  — team uses React now

UNDOCUMENTED (frequent in docs but no taxonomy category):
  "opentelemetry"    suggest: observability    45 docs (18%)
  "feature flag"     suggest: deployment       32 docs (13%)
  "service mesh"     suggest: infra            28 docs (11%)

ACTION: Create documentation for OpenTelemetry, feature flags,
and service mesh. Archive Docker Swarm and Jenkins docs.
```

### 5. Programmatic Integration

Embed Korel as a Go library in internal tools:

```go
import "github.com/cognicore/korel/pkg/korel"

// Initialize once
engine := korel.New(korel.Options{
    Store:    sqliteStore,
    Pipeline: pipeline,
    Inference: inferenceEngine,
    Weights:  korel.ScoreWeights{AlphaPMI: 1.0, BetaCats: 0.6, GammaRecency: 0.8},
})

// Embed in Slack bot, internal portal, CLI tool, etc.
results, _ := engine.Search(ctx, "how to rotate TLS certificates")
for _, card := range results.Cards {
    fmt.Printf("%s\n  %s\n  Source: %s\n\n",
        card.Title, card.Bullets[0], card.Sources[0])
}
```

No API keys. No per-query costs. No external network calls.

## Benefits

- **Replaces $120K+/year enterprise search** — Korel runs on a single VM. No per-seat licensing, no per-query API costs. The only cost is compute (one server).
- **Self-organizing** — AutoTune discovers company-specific terminology. No search admin maintaining synonym dictionaries or boost rules. New jargon is picked up automatically.
- **Content health monitoring** — Monthly drift reports tell the documentation team exactly which topics are stale and which are undocumented. Replaces manual content audits.
- **On-premise by default** — Internal documents never leave the network. No IP exposure risk from sending proprietary code, architecture decisions, or security runbooks to external APIs.
- **Multi-token precision** — "Database migration rollback" is understood as a concept, not three unrelated words. Internal compound terms ("blue-green deployment", "canary release", "circuit breaker pattern") are recognized automatically.
- **Authority-weighted ranking** — Documents linked from many other docs rank higher. A runbook referenced by 12 post-mortems outranks a draft page with zero inbound links.

## Deployment Architecture

```
Internal Doc Sources                    Engineers
  │ Confluence                            │
  │ Google Docs          ┌────────────┐   │ CLI / Slack bot / Web UI
  │ GitHub wikis    ──►  │   Korel    │ ◄─┤
  │ Markdown repos       │  (single   │   │
  │ Slack exports        │   server)  │   │
  └──────────────────►   └────────────┘   │
         │                                │
    Weekly ingest              Unlimited queries
    Monthly AutoTune           Zero per-query cost
```

Total infrastructure: one VM (4 CPU, 8GB RAM), one SQLite database. Handles 50K+ documents and serves the entire engineering organization.
