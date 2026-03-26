# Korel

  The correlation kernel for AI powered systems 

**Knowledge Organization & Retrieval Engine Library**


A research initiative by [cognicore.io](https://cognicore.io) – sistemica GmbH

---

## 🚀 Quick Start

**Get running in 2 minutes with real data!**

```bash
# Download Hacker News stories (100 articles, ~30 seconds)
go run ./cmd/korel download hn

# Or download arXiv papers (200 papers, ~60 seconds)
go run ./cmd/korel download arxiv cs.AI 200

# Index a JSONL corpus and search
# Index and search (see QUICKSTART.md for config setup)
go run ./cmd/korel index -data data/hn.jsonl -db data/hn.db \
  -stoplist configs/stoplist.yaml -dict configs/tokens.dict \
  -taxonomy configs/taxonomies.yaml
go run ./cmd/korel search -db data/hn.db -query "kubernetes security" \
  -stoplist configs/stoplist.yaml -dict configs/tokens.dict \
  -taxonomy configs/taxonomies.yaml
```

**📖 [Read the Quick Start Guide →](QUICKSTART.md)**

Available datasets:
- **Hacker News** - Tech/startup news (immediate download, ~100KB)
- **arXiv Papers** - Academic research (cs.AI, cs.CL, econ, finance, etc.)

---

## Overview

Korel is a knowledge retrieval system built on **proven statistical methods** from decades of language modeling research – before the Transformer era. It returns to the foundational principles that powered IBM's alignment models and n-gram systems in the 1990s, adapted for modern enterprise needs.

### Why Statistical Foundations?

Language modeling didn't begin with Transformers in 2017. For decades, **statistical co-occurrence models** trained on hundreds of millions of words set performance records. These systems were:

- **Explainable** - Every prediction traceable to corpus statistics
- **Data-driven** - No hand-coded rules, learned from text
- **Deterministic** - Same input, same output
- **Resource-efficient** - No GPU clusters required

Korel builds on these proven foundations while addressing modern challenges: hallucinations, black-box reasoning, and enterprise compliance requirements.

### Core Principles

Korel combines **two explainable paradigms** that complement each other:

#### 1. Statistical Foundation (1990s Language Models)
1. **Co-occurrence Analysis (PMI)** - Measures term relationships through corpus statistics (proven since 1990s)
2. **Multi-token Recognition** - Treats "machine learning" as one concept, not two words (IBM alignment models)
3. **Explicit Taxonomies** - Structured domain knowledge, not learned embeddings
4. **Transparent Scoring** - Every result explains its ranking (PMI + categories + recency + authority)
5. **Self-adjusting** - Stopword lists and term importance learned from data patterns

#### 2. Symbolic Reasoning (1980s Expert Systems)
6. **Logical Inference Engine** - Pure Go rule engine for domain knowledge
   - Query expansion via `is_a`, `used_for`, `related_to` relations
   - Multi-hop BFS expansion with 0.7× confidence decay per hop (pruned below 0.3)
   - Transitive reasoning: if `bert is_a transformer` and `transformer is_a neural-network`, then `bert is_a neural-network`
   - Explainable proof chains: shows exact logical steps
   - Auto-populated from AutoTune: high-PMI pairs become `related_to` facts
   - Swappable interface: start simple, upgrade to full Prolog if needed

**Example:**
```
Query: "bert"
Statistical: Finds documents via PMI co-occurrence
Symbolic: Expands to [bert, transformer, neural-network, attention-mechanism]
Result: More comprehensive retrieval with logical explanations
```

**The Result:**
- ✅ No hallucinations (only returns what exists in corpus)
- ✅ Full explainability (shows PMI scores AND logical proof chains)
- ✅ GPU-optional (statistics + logic, no neural networks)
- ✅ Enterprise-ready (security, compliance, IP control)
- ✅ **Multi-hop reasoning** (connects concepts through logical inference)

### Comparison: Neural vs. Symbolic+Statistical

| Aspect | Neural RAG (Transformers) | Korel (Statistical + Symbolic) |
|--------|---------------------------|--------------------------------|
| Retrieval | Vector similarity (black box) | PMI + logical inference (transparent) |
| Multi-words | Separate tokens | Recognized as phrases |
| Query expansion | Learned embeddings | Logical rules (is_a, related_to) |
| Explainability | "Embedding matched" | PMI scores + proof chains |
| Multi-hop reasoning | Implicit in embeddings | Explicit logical paths |
| Hardware | GPU clusters | CPU sufficient |
| Hallucinations | Common (generates text) | None (retrieves only) |
| Training | Weeks on GPUs | Hours on CPUs + rule definition |

### The Vision

Korel demonstrates that **1980s symbolic AI + 1990s statistical NLP** can deliver enterprise AI without:
- Massive GPU infrastructure
- Black-box decision making
- Unpredictable hallucinations
- Token-based pricing models

**The Innovation:** Instead of choosing between statistical and symbolic approaches, Korel combines both:
- **Statistical layer** discovers patterns from data (PMI, co-occurrence)
- **Symbolic layer** encodes domain knowledge (taxonomies, rules)
- **Together** they provide explainable, multi-hop reasoning

Inspired by decades of proven research (IBM n-grams, expert systems, "web as corpus") and recent explorations by researchers like Vincent Granville, Korel asks: *Can pre-Transformer approaches solve enterprise AI challenges better than neural architectures?*

---

## Architecture

### 8-Phase Pipeline

```
┌─────────────────────────────────────────────────────────┐
│  Phase 1: EXTRACT                                       │
│  ├─ Crawl RSS/feeds/documents                          │
│  ├─ Normalize & deduplicate                            │
│  └─ Output: raw docs (URL, title, text, links)         │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 2: TRANSFORM I – Tokenization                    │
│  ├─ Split into tokens                                   │
│  ├─ Remove stopwords/fillers (self-adjusting list)     │
│  └─ Output: clean token streams                         │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 3: TRANSFORM II – Multi-Token & Lexicon          │
│  ├─ Greedy longest-match for phrases                   │
│  ├─ Lexicon normalization (ML→ml, gaming→game)         │
│  ├─ Synonym variants (FiT → feed-in tariff)            │
│  └─ Output: normalized concept tokens                   │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 4: TRANSFORM III – Taxonomy Tagging              │
│  ├─ Assign categories (policy, solar, finance...)      │
│  ├─ Extract entities (tickers, countries, dates)       │
│  └─ Output: enriched documents                          │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 5: TRANSFORM IV – PMI Calculation                │
│  ├─ Count Nx (term frequency)                          │
│  ├─ Count Nxy (co-occurrence)                          │
│  ├─ Calculate PMI with ε-smoothing                     │
│  └─ Output: token pair scores                           │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 6: LOAD – Index & Store                          │
│  ├─ SQLite with WAL mode                                │
│  ├─ Nested hash: token→docs, token→neighbors           │
│  └─ Output: queryable index                             │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 7: RETRIEVAL & RANKING                           │
│  ├─ Parse query → tokens + categories                  │
│  ├─ Fetch candidates (exact + PMI neighbors)           │
│  ├─ Hybrid score: α·PMI + β·cats + γ·recency + ...     │
│  └─ Output: ranked document list                        │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 8: SYNTHESIS – Cards & Explainability            │
│  ├─ Build structured "cards" (bullets + sources)       │
│  ├─ Score breakdown (PMI: 1.45, cats: 0.67, ...)       │
│  ├─ Self-tuning: suggest stopwords, taxonomy updates   │
│  └─ Output: explainable results                         │
└─────────────────────────────────────────────────────────┘
```

### Autotune & Maintenance Loop

- **Autotune modules** (`pkg/korel/autotune/*`) analyze stored stats after ingestion to
  propose new stopwords, taxonomy keywords, symbolic rules, or entity entries. They can
  run fully automatic or hand suggestions to a human/LLM reviewer before committing.
- **Maintenance jobs** (`pkg/korel/maintenance`) reprocess documents through the updated
  pipeline to strip newly discovered stopwords and apply taxonomy/entity changes. The
  cleaner re-tokenizes each document's title and body snippet, updating stored tokens and
  categories when they differ from the current pipeline output.

### Bootstrapping a New Corpus

1. Run the bootstrap CLI to analyze a raw JSONL corpus and emit starter configs:
   ```bash
   go run ./cmd/korel bootstrap \
     -input testdata/hn/docs.jsonl \
     -domain tech \
     -output configs/tech
   ```
   (Use `-llm-base/-llm-model` to have an OpenAI/Azure/Ollama reviewer vet stopwords.)
2. Inspect generated configs:
   - `bootstrap-report.json` - Full statistics and analysis
   - `stoplist.yaml` - Discovered stopwords
   - `synonyms.yaml` - Synonym candidates (requires manual review)
   - `tokens.dict` - Multi-token phrases
   - `taxonomies.yaml` - Category structure
3. Ingest the corpus with those configs, then rely on the autotune + maintenance loop for continuous refinement.
4. Need a lightweight DF/entropy snapshot only? `korel analyze` provides that report without emitting files.

### AI Agents & RAG

- Korel is a natural retrieval component inside tool-enabled agents.  Call `Search`
  to obtain explainable cards, then feed those facts to an LLM for synthesis.
- `korel search` now supports optional OpenAI-compatible endpoints, demonstrating how
  agents can combine Korel's deterministic retrieval with neural generation.
- See [`docs/AGENT_INTEGRATION.md`](docs/AGENT_INTEGRATION.md) for detailed patterns,
  including MCP/tool wiring and suggestions for prompt templates.
- See [`docs/TYPED-EXPANSION.md`](docs/TYPED-EXPANSION.md) for typed query expansion
  (synonym, broader, narrower) using distributional relationship classification.

### Hybrid Architecture: Statistical + Symbolic

Korel's innovation is the **integration layer** between statistical discovery and symbolic reasoning:

```
┌─────────────────────────────────────────────────────────────┐
│                    QUERY: "bert"                             │
└──────────────────────┬──────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          │                         │
    ┌─────▼─────┐           ┌──────▼──────┐
    │ SYMBOLIC  │           │ STATISTICAL │
    │ REASONING │           │  ANALYSIS   │
    └─────┬─────┘           └──────┬──────┘
          │                         │
    Inference Rules:          PMI Scores:
    bert is_a transformer     bert ↔ transformer: 2.3
    transformer is_a NN       bert ↔ nlp: 1.8
    transformer used_for nlp  transformer ↔ attention: 2.1
          │                         │
    ┌─────▼─────────────────────────▼──────┐
    │   EXPANDED QUERY                     │
    │   [bert, transformer, neural-network,│
    │    nlp, attention-mechanism]         │
    └────────────┬─────────────────────────┘
                 │
    ┌────────────▼──────────────┐
    │  HYBRID SCORING           │
    │  α·PMI + β·cats + ...     │
    │  + θ·inference_strength   │
    └────────────┬──────────────┘
                 │
    ┌────────────▼──────────────┐
    │  EXPLAINABLE CARD         │
    │  • PMI: 2.3               │
    │  • Inference: bert→NN     │
    │  • Proof chain shown      │
    └───────────────────────────┘
```

**Key Insight:**
- PMI discovers "bert" and "transformer" co-occur frequently (but doesn't know *why*)
- Symbolic rules explain "bert is_a transformer" (but don't discover new patterns)
- **Together:** Data-driven discovery + logical explanation = explainable AI

---

## Project Structure

### Core Library (`pkg/korel/`)

The reusable Go library providing the core functionality:

```
pkg/korel/
├── korel.go              # Public API facade (Ingest, Search, AutoTune, RebuildPipeline)
├── search.go             # SearchRequest, ExpandMode, expansion logic
├── graph.go              # Graph building, edge classification, WarmInference
├── mode.go               # SearchMode detection (Fact/Trend/Compare/Explore)
├── rewrite.go            # Query rewriting (dictionary-based canonicalization)
├── feedback.go           # User feedback recording & stats
├── store/                # Storage interface + implementations
│   ├── store.go          #    - Interface: Store, StoplistView, DictView, TaxonomyView
│   ├── memstore/         #    - In-memory impl (interned int32 keys, lazy adjacency)
│   └── sqlite/           #    - SQLite impl (WAL, 13+ tables incl. edges, feedback)
├── ingest/               # Tokenization, multi-token, taxonomy
├── lexicon/              # Synonym normalization & c-token relationships
├── analytics/            # Corpus analysis (parallel ProcessBatch, fused ComputeAll)
├── pmi/                  # Co-occurrence counting & PMI/NPMI calc
├── rank/                 # Hybrid scoring (PMI + BM25 + title + damping)
├── cards/                # Card synthesis & explainability
├── query/                # Query parsing & retrieval
├── reltype/              # Distributional relationship classifier (synonym/broader/narrower)
├── inference/            # Symbolic reasoning engine
│   ├── inference.go      #    - Interface for swappable engines
│   ├── simple/           #    - Pure Go engine (multi-hop BFS, confidence decay)
│   └── prolog/           #    - Full Prolog engine (directional typed expansion rules)
├── signals/              # Density damping, collision detection, prediction error
├── stoplist/             # Self-adjusting stopword management
├── autotune/             # Iterative stopword + rule + taxonomy discovery
│   ├── stopwords/        #    - Stopword candidate detection
│   ├── rules/            #    - PMI→rule auto-generation
│   ├── entities/         #    - Entity extraction tuning
│   └── taxonomy/         #    - Taxonomy drift detection & refinement
├── maintenance/          # Partial reindex, rule export
└── config/               # Config loaders (YAML)
```

### Use Cases

**1. Indexer** (`korel index`)
- Ingests documents into Korel
- Updates PMI scores incrementally

**2. Search** (`korel search`)
- Interactive Q&A interface (or one-shot with `-query`)
- SearchMode detection adjusts weights per query intent
- Typed expansion via ExpandMode (synonym, broader, narrower)
- Shows explainable cards with sources

**3. Graph Explorer** (`korel graph`)
- Interactive REPL for navigating the knowledge graph
- Browse typed edges (is_a, related_to, synonym, broader/narrower)
- Inspect relationship classifications from the `reltype` package

### Test Data (`testdata/`)

Configuration files and downloaded corpora for testing:

```
testdata/
├── hn/                  # Hacker News domain configs
│   ├── stoplist.yaml    #   Stopword list (self-adjusting)
│   ├── tokens.dict      #   Multi-token dictionary
│   └── taxonomies.yaml  #   Sectors, events, regions, entities
├── arxiv/               # arXiv domain configs
│   ├── stoplist.yaml
│   ├── tokens.dict
│   └── taxonomies.yaml
├── bootstrap/           # Bootstrap command configs
│   ├── stoplist.yaml
│   ├── tokens.dict
│   └── taxonomies.yaml
└── chat/
    └── queries.txt      # Sample queries for testing
```

Corpus data (`docs.jsonl`) is created by the download commands — see Quick Start.

---

## Key Concepts

### Multi-Token Recognition

Traditional tokenizers break "feed-in tariff" into three meaningless words.
Korel treats it as **one semantic unit** using a greedy longest-match algorithm.

```
Input:  ["government", "considers", "feed-in", "tariff", "for", "solar"]
Output: ["government", "considers", "feed-in tariff", "solar"]
```

### PMI (Pointwise Mutual Information)

Measures how strongly two terms co-occur beyond random chance:

```
PMI(a,b) = log((N_ab + ε) · N / ((N_a + ε)(N_b + ε)))
```

Where:
- `N_ab` = documents containing both a and b
- `N_a`, `N_b` = documents containing each term
- `ε` = smoothing constant (typically 1.0)

High PMI → strong semantic relationship (e.g., "solar" ↔ "feed-in tariff")
Low PMI → weak/random co-occurrence

### Hybrid Scoring

Unlike pure vector similarity, Korel ranks results using transparent weights:

```
score = α·PMI·damping + ζ·BM25 + ι·title + β·category_overlap + γ·recency + η·authority + θ·inference - δ·length_penalty
```

Default weights (tunable):
- α = 1.0 (PMI importance, scaled by per-token damping factor)
- ζ = 0.4 (BM25 term relevance, K1=1.2, B=0.35)
- ι = 0.3 (title match boost)
- β = 0.6 (category matching)
- γ = 0.8 (recency, exponential decay)
- η = 0.2 (link authority)
- θ = 0.3 (symbolic inference strength)
- δ = 0.05 (length normalization)

Hub tokens that connect to many neighbors get density-based damping (smoothstep curve, floor 0.1) so they contribute less PMI signal — preventing generic terms from dominating results.

### Symbolic Reasoning

Korel includes a **pure Go inference engine** for logical query expansion:

**Rule Format** (`configs/rules/ai.rules`):
```prolog
# Taxonomy
is_a(bert, transformer)
is_a(transformer, neural-network)
is_a(neural-network, model)

# Usage
used_for(transformer, nlp)
requires(transformer, attention-mechanism)

# Relations
related_to(bert, masked-language-modeling)
alternative_to(lstm, transformer)
```

**How It Works:**
1. **Transitive Closure**: If `bert is_a transformer` and `transformer is_a neural-network`, infer `bert is_a neural-network`
2. **Query Expansion**: Query "bert" expands to `[bert, transformer, neural-network, attention-mechanism]`
3. **Proof Chains**: Every inference step is recorded and shown to user
4. **Swappable**: Two engines available — simple Go BFS engine and full Prolog engine with directional typed expansion rules

**Example Query Flow:**
```
User Query: "bert"

Symbolic Engine:
  bert is_a transformer (direct fact)
  transformer is_a neural-network (direct fact)
  transformer used_for nlp (direct fact)
  → Expanded: [bert, transformer, neural-network, nlp, attention-mechanism]

Statistical Engine:
  bert ↔ transformer: PMI=2.3 (high co-occurrence)
  bert ↔ nlp: PMI=1.8 (moderate)

Combined Result:
  Documents about "transformer architecture for NLP" rank high
  Explanation shows BOTH statistical evidence AND logical reasoning
```

### Explainable Cards

Instead of generated text, Korel returns structured cards:

```json
{
  "card_id": "C-77",
  "title": "Feed-in tariff ↔ Solar (Week 45, Italy)",
  "bullets": [
    "Italy considers new feed-in tariff for rooftop solar",
    "Utilities plan higher solar CAPEX for 2026"
  ],
  "sources": [
    {"url": "https://example.com/policy/italy-fit", "time": "2025-11-09T07:30:00Z"}
  ],
  "score_breakdown": {
    "pmi": 1.28,
    "cats": 0.50,
    "recency": 0.39,
    "authority": 0.18,
    "len": 0.06
  },
  "explain": {
    "query_tokens": ["feed-in tariff", "solar", "italy"],
    "top_pairs": [["feed-in tariff", "policy", 1.80], ["solar", "feed-in tariff", 1.10]]
  }
}
```

### Self-Adjusting Stoplist

The system monitors token statistics and suggests stopword candidates:

| Token | DF% | PMI_max | Cat_Entropy | Suggest Drop? |
|-------|-----|---------|-------------|---------------|
| announced | 84% | 0.02 | 0.95 | ✅ (generic) |
| subsidy | 14% | 1.42 | 0.23 | ❌ (meaningful) |
| company | 91% | 0.01 | 0.98 | ✅ (filler) |

Criteria for removal:
- Low IDF (appears everywhere)
- Low PMI_max (no strong associations)
- High category entropy (uniform distribution)

---

## Getting Started

### Prerequisites

- Go 1.22+
- SQLite (via `modernc.org/sqlite`, no CGO required)
- Internet connection (for downloading corpora)

### Full Testing Workflow (Step-by-Step)

This guide walks through the complete process of testing Korel with real data.

#### Step 1: Initialize Project

```bash
cd korel
go mod tidy
```

#### Step 2: Download Test Corpora

Download 50 Hacker News stories and 50 arXiv AI papers:

```bash
# Download Hacker News tech stories (50 documents)
go run ./cmd/korel download hn 50

# Download arXiv AI research papers (50 documents)
go run ./cmd/korel download arxiv cs.AI 50
```

**Output:**
- `testdata/hn/docs.jsonl` - 50 HN stories
- `testdata/arxiv/docs.jsonl` - 50 arXiv papers

**Verify downloads:**
```bash
wc -l testdata/hn/docs.jsonl testdata/arxiv/docs.jsonl
# Should show: 50 + 50 = 100 total documents
```

#### Step 3: Ingest Hacker News Corpus

```bash
# Ingest HN corpus into database
go run ./cmd/korel index \
  -db ./data/hn.db \
  -data testdata/hn/docs.jsonl \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -rules configs/rules/ai.rules    # optional: symbolic inference rules
```

**What happens:**
1. Creates SQLite database at `./data/hn.db`
2. Tokenizes documents (removes stopwords)
3. Recognizes multi-token phrases (e.g., "machine learning")
4. Assigns categories based on taxonomy
5. Builds PMI co-occurrence statistics
6. Takes ~5-10 seconds for 50 documents

**Expected output:**
```
Korel RSS Indexer started
Loaded 50 documents from testdata/hn/docs.jsonl
Ingested 10/50 documents
Ingested 20/50 documents
...
✓ Indexing complete: 50 documents processed
```

#### Step 4: Ingest arXiv Corpus

```bash
# Ingest arXiv corpus into separate database
go run ./cmd/korel index \
  -db ./data/arxiv.db \
  -data testdata/arxiv/docs.jsonl \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**Result:** Second database at `./data/arxiv.db` with 50 research papers indexed.

#### Step 5: Query the Databases

Now test retrieval with interactive queries:

**Query Hacker News corpus:**
```bash
go run ./cmd/korel search \
  -db ./data/hn.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -rules configs/rules/ai.rules    # optional: symbolic inference rules
```

**Example queries to try:**
```
> open source project
> startup funding
> security vulnerability
> programming language
> web browser
```

**Query arXiv corpus:**
```bash
go run ./cmd/korel search \
  -db ./data/arxiv.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**Example queries to try:**
```
> machine learning
> computer vision
> neural network
> natural language processing
> reinforcement learning
```

**One-shot query mode (non-interactive):**

For testing and automation, you can execute a single query without entering interactive mode:

```bash
# Query with default topK=3
go run ./cmd/korel search \
  -db ./data/hn.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -query "open source"

# Query with custom topK
go run ./cmd/korel search \
  -db ./data/arxiv.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -query "machine learning" \
  -topk 5
```

This is useful for:
- Automated testing and CI/CD pipelines
- Quick verification after ingestion
- Scripting and batch processing
- Comparing results across different queries

#### Step 6: Understand the Results

Each query returns a **Card** with:

```
--- Card 1: [Title] ---
  • Bullet point 1 (key finding)
  • Bullet point 2 (key finding)

Sources:
  - https://example.com/article (2025-11-10)

Score Breakdown:
  pmi: 1.45        # Co-occurrence score
  cats: 0.67       # Category match
  recency: 0.45    # Time decay
  authority: 0.25  # Link authority
  len: 0.05        # Length penalty

Explain:
  Query tokens: [machine, learning]
  Expanded tokens: [machine, learning, neural, deep]
  Matched tokens: [machine, learning]
  Top pairs:
    machine ↔ learning (PMI: 2.15)
    learning ↔ deep (PMI: 1.80)
```

**What each field means:**
- **Bullets:** Key sentences from matching documents
- **Sources:** Original URLs with timestamps
- **Score Breakdown:** How each ranking factor contributed
- **Explain:** Shows query expansion and PMI relationships

#### Step 7: Verify PMI Statistics

Check that co-occurrence statistics were built:

```bash
# Check database size (should be >100KB for 50 docs)
ls -lh data/hn.db data/arxiv.db

# Count documents in database (should show 50 each)
sqlite3 data/hn.db "SELECT COUNT(*) FROM docs;"
sqlite3 data/arxiv.db "SELECT COUNT(*) FROM docs;"

# Check token pairs (PMI statistics)
sqlite3 data/hn.db "SELECT COUNT(*) FROM token_pairs;"
# Should show many pairs (depends on document diversity)
```

#### Step 8: Compare Domain Differences

Test the same query across both corpora to see different results:

**Query:** "ai" in both databases

**HN result:** News about AI startups, products, discussions
**arXiv result:** Academic papers about AI algorithms, research

This demonstrates **domain-specific retrieval** - same query, different contexts.

---

### Quick Start (Pre-existing Data)

If you already have indexed data:

```bash
# Query existing database
go run ./cmd/korel search \
  -db ./data/hn.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

---

### Example Session

```
> machine learning neural networks

--- Card 1: Machine Learning Basics ---
  • Machine learning is a subset of artificial intelligence
  • Uses neural networks and deep learning techniques

Sources:
  - https://example.com/ml-basics (2025-11-09)

Score Breakdown:
  pmi: 1.45
  cats: 0.67
  recency: 0.45
  authority: 0.25

Explain:
  Query tokens: [machine learning, neural networks]
  Expanded tokens: [machine learning, neural networks, deep learning, ai]
  Top pairs:
    machine learning ↔ neural networks (PMI: 2.35)
```

---

## Configuration

### Core Config (`configs/korel.yaml`)

```yaml
db_path: ./data/korel.db
snapshot_dir: ./data/snapshots
dict_path: testdata/hn/tokens.dict
taxonomy_path: testdata/hn/taxonomies.yaml
stoplist_path: testdata/hn/stoplist.yaml

score_weights:
  alpha_pmi: 1.0
  zeta_bm25: 0.4
  iota_title: 0.3
  beta_cats: 0.6
  gamma_recency: 0.8
  eta_authority: 0.2
  delta_len: 0.05

recency_halflife_days: 14
```

### Token Dictionary (`testdata/hn/tokens.dict`)

```
feed-in tariff|fit|policy
capital expenditure|capex|finance
photovoltaics|pv|solar
power purchase agreement|ppa|finance
```

Format: `canonical_form|synonym1|synonym2|category`

### Taxonomy (`testdata/hn/taxonomies.yaml`)

```yaml
sectors:
  solar: [solar, photovoltaics, pv, rooftop]
  wind: [wind, offshore, onshore]

events:
  policy: [feed-in tariff, subsidy, tender, regulation]
  finance: [capital expenditure, capex, earnings, dividend]

regions:
  italy: [italy, rome, ita]

entities:
  tickers:
    ENEL: [enel, enel spa]
```

### Inference Rules (`configs/rules/ai.rules`)

Optional Prolog-style rules for symbolic query expansion:

```prolog
is_a(bert, transformer)
is_a(transformer, neural-network)
used_for(transformer, nlp)
related_to(bert, masked-language-modeling)
```

The `korel index` and `korel search` subcommands accept an optional `-rules` flag. AutoTune also auto-generates `related_to` rules from high-PMI pairs.

---

## Testing

```bash
# Run all tests
go test ./pkg/korel/... -timeout 300s

# TinyStories AutoTune benchmark (5k stories, ~7s)
go test ./pkg/korel/ -run TinyStoriesAutoTune -v -timeout 120s

# Full corpus benchmark (22k stories, ~38s)
# Edit tinystories_test.go: set numStories = 0

# Run specific packages
go test ./pkg/korel/analytics/ -v
go test ./pkg/korel/inference/simple/ -v
go test ./pkg/korel/store/sqlite/ -v

# Integration test (indexer → query)
./scripts/test_e2e.sh
```

---

## Performance

Benchmarked on the TinyStories corpus (simple English narratives) using iterative AutoTune with density-based damping:

| Corpus | Stories | Time | Semantic Hits | Noise | Rules Discovered |
|--------|---------|------|---------------|-------|-----------------|
| Subset | 5,000 | 6.8s | 6/10 | 0 | 84 |
| Full | 21,989 | 37.5s | 10/10 | 0 | 144 |

**Scaling:** Linear time, better-than-linear quality. At full corpus, PMI neighbors are richer (e.g., `ball → [kick, soccer, baseball, bat, golf, bounce, throw]`) and rule discovery improves 70% (e.g., `barber→haircut`, `cream→ice`, `needle→sew`, `kings→queens`).

**AutoTune convergence:** 2 rounds regardless of corpus size. Discovered stopwords are stable across sizes (happy, saw, said — corpus-specific high-frequency low-information tokens beyond the 84 base stopwords).

**Optimization history:**
- Baseline (naive): 18.3s @ 5k stories
- Token interning + fused iteration + parallel batch: 6.8s (63% faster)
- Key techniques: integer pair keys (`[2]int32`), lazy adjacency index, fused `ComputeAll()` replacing 3 separate map iterations, parallel `ProcessBatch` with per-worker local counts

---

## Roadmap

### Phase 1: Core Library
- ✅ SQLite storage with WAL
- ✅ Multi-token recognition
- ✅ PMI calculation with ε-smoothing (NPMI supported)
- ✅ Hybrid scoring (PMI + categories + recency + authority + inference)
- ✅ Card synthesis
- ✅ Symbolic inference engine (pure Go, swappable interface)

### Phase 2: Self-Tuning
- ✅ Iterative AutoTune with automatic stopword detection
- ✅ PMI→Rules auto-generation (high-PMI pairs become `related_to` facts)
- ✅ AutoTune → Store persistence (stopwords + rules survive restarts)
- ✅ RebuildPipeline reads back stoplist/dict/taxonomy from store
- ✅ Density-based damping (hub tokens get reduced PMI influence)
- ✅ Multi-hop inference (BFS expansion with confidence decay)
- ✅ Full SQLite parity (stoplist, dict, taxonomy tables + views)
- ✅ Taxonomy drift detection (`autotune/taxonomy` — coverage + orphan detection)
- [ ] LLM-assisted synonym expansion

### Phase 2b: Retrieval Intelligence
- ✅ BM25 term-relevance scoring (K1/B tunable, fused into hybrid formula)
- ✅ Title match boost (separate weight, tokens matched against query)
- ✅ SearchMode detection (Fact / Trend / Compare / Explore with per-mode weight multipliers)
- ✅ Typed expansion (`reltype` classifier: synonym / broader / narrower / related)
- ✅ ExpandMode in search (select expansion strategy: synonym-only, broader, narrower, typed)
- ✅ Prolog inference engine (`inference/prolog` — full engine alongside simple Go engine)
- ✅ Query rewriting (dictionary-based synonym canonicalization before search)
- ✅ Graph exploration REPL (`korel graph` — interactive edge navigation)
- ✅ Feedback recording (click/dismiss events, aggregated stats for adaptive ranking)
- ✅ Output formats (cards, briefing, memo, digest, watchlist)

### Phase 3: Production
- [ ] PostgreSQL backend
- [ ] Incremental PMI updates (ΔPMI)
- [ ] Distributed indexing
- [ ] Web UI for cards & ΔPMI dashboard

### Phase 4: Agent Integration
- [ ] Dual-agent architecture (Dialog + Memory Curator)
- [ ] Event-driven knowledge updates
- [ ] Trust scores & low-confidence warnings

---

## Why Korel?

**For researchers:**
- Revisit statistical NLP foundations with modern tooling
- Study co-occurrence patterns in domain-specific corpora
- Build explainable AI systems without neural black boxes

**For enterprises:**
- No GPU required (runs on modest hardware)
- Fully offline/on-premise capable
- Audit trails for compliance (every retrieval is explained)
- No hallucinations (only returns what exists in corpus)

**For developers:**
- Clean Go library with minimal dependencies
- Easy to integrate with existing LLM pipelines
- Testable, deterministic behavior

---

## Historical Foundations & Related Work

Korel builds on decades of proven research:

**Statistical Language Modeling (1990s-2000s):**
- IBM's n-gram models and alignment systems
- Smoothing techniques (Kneser-Ney, Good-Turing)
- "Web as corpus" approaches
- PMI and co-occurrence analysis

**Modern Revival:**
- Vincent Granville's xLLM architecture (enterprise statistical models)
- Hybrid search systems (Vespa, Elasticsearch)
- Explicit knowledge graphs (Neo4j, Dgraph)

**Related cognicore.io Research:**
- Spiking neural networks
- Agentigo
- Golog / Prolog in Go

---

## License

MIT (pending – research project)

---

## Contact

**cognicore.io** – sistemica GmbH
Research & Development in alternative AI architectures

Exploring whether statistical foundations can deliver better enterprise AI than Transformer-based systems.

For questions or collaboration: [contact via sistemica.de]

---

## Korel's Innovation

While building on proven foundations, Korel contributes **new research**:

### 1. **Hybrid Statistical-Symbolic Architecture**
First system to integrate 1990s statistical methods (PMI) with 1980s symbolic AI (rule engines) in a unified retrieval pipeline. Neither alone is sufficient:
- Statistics discover patterns but can't explain relationships
- Symbols encode knowledge but can't discover new patterns
- **Together** they provide both discovery and explainability

### 2. **Dual Inference Engines in Pure Go**
Two symbolic reasoning engines, both in pure Go:
- **Simple engine**: multi-hop BFS with 0.7× confidence decay, ideal for fast expansion
- **Prolog engine**: full rule engine with directional typed expansion (`expand_synonym`, `expand_broader`, `expand_narrower`)
- Swappable via common interface — choose per use case
- Transitive closure for taxonomies
- Proof chain generation
- No external dependencies (no Python/shell bridges)

### 3. **Explainable Hybrid Scoring**
Extended transparent scoring formula:
```
score = α·PMI + ζ·BM25 + ι·title + β·categories + γ·recency + η·authority + θ·inference - δ·length
```
Every component is measurable, tunable, and explainable to auditors.

### 4. **Self-Learning Rules**
AutoTune uses PMI statistics to auto-generate symbolic rules:
- High PMI pairs with confidence ≥ 0.6 become `related_to(X, Y)` facts
- Rules are persisted to store and fed into the inference engine via `AddFact`
- Multi-hop expansion (BFS, 0.7× confidence decay per hop) makes discovered rules usable in query expansion
- Example: corpus discovers `cream→ice`, `needle→sew`, `barber→haircut` without any manual rule writing

**The Result:** A research platform demonstrating that pre-Transformer approaches, properly integrated, can solve modern enterprise AI challenges without GPU infrastructure or black-box reasoning.

---

## Acknowledgments

Inspired by the statistical NLP revolution of the 1990s-2000s (IBM, web-scale corpus methods), symbolic AI systems (Prolog, expert systems), and recent explorations by researchers like Vincent Granville who demonstrate that pre-Transformer approaches remain relevant for enterprise applications requiring explainability, determinism, and resource efficiency.
