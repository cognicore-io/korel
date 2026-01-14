# Korel Quick Start Guide

Get Korel running in **under 2 minutes** with real data!

---

## Prerequisites

```bash
cd korel
go mod tidy
```

---

## 🚀 2-Minute Bootstrap Workflow

### Step 1: Download Data (~30 seconds)

```bash
# Download Hacker News stories
go run ./cmd/download-hn 50

# Or download arXiv papers
go run ./cmd/download-arxiv cs.AI 50
```

**Output:** `testdata/hn/docs.jsonl` or `testdata/arxiv/docs.jsonl`

### Step 2: Bootstrap Analytics (~10 seconds)

Analyze the corpus to discover stopwords and high-frequency tokens:

```bash
# Run analytics with minimal bootstrap configs
go run ./cmd/korel-analytics \
  --input=testdata/hn/docs.jsonl \
  --stoplist=testdata/bootstrap/stoplist.yaml \
  --dict=testdata/bootstrap/tokens.dict \
  --taxonomy=testdata/bootstrap/taxonomies.yaml
```

**Output:**
```json
{
  "total_docs": 50,
  "stopword_candidates": [
    {"token": "source", "score": 0.79},
    {"token": "https", "score": 0.79}
  ],
  "high_df_tokens": [
    {"token": "source", "df_percent": 98, "entropy": 0},
    {"token": "https", "df_percent": 96, "entropy": 0},
    {"token": "com", "df_percent": 54, "entropy": 0}
    ...
  ]
}
```

**What to do:** Review the suggestions and use them to seed your configs. The configs in `testdata/hn/` are already seeded for you.

### Step 3: Ingest (~10 seconds for 50 docs)

```bash
# Ingest HN corpus
go run ./cmd/rss-indexer \
  --db=./data/hn.db \
  --data=testdata/hn/docs.jsonl \
  --stoplist=testdata/hn/stoplist.yaml \
  --dict=testdata/hn/tokens.dict \
  --taxonomy=testdata/hn/taxonomies.yaml
```

**Output:**
```
2025/11/10 14:46:07 Korel RSS Indexer started
2025/11/10 14:46:07 Loaded 50 documents from testdata/hn/docs.jsonl
2025/11/10 14:46:07 Ingested 10/50 documents
...
2025/11/10 14:46:07 ✓ Indexing complete: 50 documents processed
```

### Step 4: Query (instant)

```bash
# One-shot query
go run ./cmd/chat-cli \
  --db=./data/hn.db \
  --stoplist=testdata/hn/stoplist.yaml \
  --dict=testdata/hn/tokens.dict \
  --taxonomy=testdata/hn/taxonomies.yaml \
  --query="open source"

# Interactive mode
go run ./cmd/chat-cli \
  --db=./data/hn.db \
  --stoplist=testdata/hn/stoplist.yaml \
  --dict=testdata/hn/tokens.dict \
  --taxonomy=testdata/hn/taxonomies.yaml
```

**Example queries to try:**
```
> open source
> machine learning
> security
> startup
```

---

## 🎓 Alternative: arXiv Papers

### Full workflow for arXiv:

```bash
# 1. Download
go run ./cmd/download-arxiv cs.AI 50

# 2. Bootstrap (if starting from scratch)
go run ./cmd/korel-analytics \
  --input=testdata/arxiv/docs.jsonl \
  --stoplist=testdata/bootstrap/stoplist.yaml \
  --dict=testdata/bootstrap/tokens.dict \
  --taxonomy=testdata/bootstrap/taxonomies.yaml

# 3. Ingest (using pre-seeded configs)
go run ./cmd/rss-indexer \
  --db=./data/arxiv.db \
  --data=testdata/arxiv/docs.jsonl \
  --stoplist=testdata/arxiv/stoplist.yaml \
  --dict=testdata/arxiv/tokens.dict \
  --taxonomy=testdata/arxiv/taxonomies.yaml

# 4. Query
go run ./cmd/chat-cli \
  --db=./data/arxiv.db \
  --stoplist=testdata/arxiv/stoplist.yaml \
  --dict=testdata/arxiv/tokens.dict \
  --taxonomy=testdata/arxiv/taxonomies.yaml \
  --query="machine learning"
```

**Good arXiv queries:**
- `transformer architecture`
- `neural network optimization`
- `reinforcement learning`
- `computer vision`

---

## 📊 Understanding the Results

Each query returns **explainable cards**:

```
--- Card 1: [Title] ---
  • Key finding 1
  • Key finding 2

Sources:
  - https://example.com/article (2025-11-10)

Score Breakdown:
  pmi: 1.45        # Co-occurrence strength
  cats: 0.67       # Category match
  recency: 0.45    # Time decay
  authority: 0.25  # Link count
  len: 0.05        # Length normalization

Explain:
  Query tokens: [machine, learning]
  Expanded tokens: [machine, learning]  # + symbolic inference
  Matched tokens: [machine, learning]
  Top pairs:
    machine ↔ learning (PMI: 2.15)
```

---

## 🔧 Optional: LLM Review (Bootstrap Enhancement)

Add LLM review to vet stopword candidates:

```bash
go run ./cmd/korel-analytics \
  --input=testdata/hn/docs.jsonl \
  --stoplist=testdata/bootstrap/stoplist.yaml \
  --dict=testdata/bootstrap/tokens.dict \
  --taxonomy=testdata/bootstrap/taxonomies.yaml \
  --llm-base="http://localhost:11434/v1" \
  --llm-model="llama2" \
  --llm-api-key=""
```

Works with:
- **OpenAI:** `--llm-base="https://api.openai.com/v1" --llm-model="gpt-4"`
- **Ollama:** `--llm-base="http://localhost:11434/v1" --llm-model="llama2"`
- **Azure:** See `configs/ai-chat.azure.yaml`

---

## 🧪 Verify Ingestion

```bash
# Check document count
sqlite3 data/hn.db "SELECT COUNT(*) FROM docs;"
# Expected: 50

# Check token statistics
sqlite3 data/hn.db "SELECT COUNT(*) FROM token_df;"
# Expected: ~100-200 unique tokens

# Check PMI pairs
sqlite3 data/hn.db "SELECT COUNT(*) FROM token_pairs;"
# Expected: ~1000-5000 pairs

# Top tokens
sqlite3 data/hn.db "SELECT token, df FROM token_df ORDER BY df DESC LIMIT 10;"
```

---

## 📁 Config Directory Structure

```
testdata/
├── bootstrap/              # Minimal configs for initial analysis
│   ├── stoplist.yaml      # Just universal stopwords (the, a, and...)
│   ├── tokens.dict        # Empty
│   └── taxonomies.yaml    # Empty sectors
│
├── hn/                    # Hacker News domain (pre-seeded)
│   ├── stoplist.yaml      # Tech news stopwords
│   ├── tokens.dict        # Tech phrases (open source, machine learning...)
│   ├── taxonomies.yaml    # Tech categories (ai, programming, security...)
│   └── docs.jsonl         # Downloaded data
│
└── arxiv/                 # arXiv academic domain (pre-seeded)
    ├── stoplist.yaml      # Academic stopwords
    ├── tokens.dict        # Academic phrases
    ├── taxonomies.yaml    # Academic categories
    └── docs.jsonl         # Downloaded papers
```

---

## 🚧 Starting from Scratch with a New Domain

### 1. Create minimal configs:

```bash
mkdir -p configs/mydomain
```

**configs/mydomain/stoplist.yaml:**
```yaml
terms:
  - the
  - a
  - an
  - and
  - or
  - in
  - on
```

**configs/mydomain/tokens.dict:**
```
# Empty - will be populated after analysis
```

**configs/mydomain/taxonomies.yaml:**
```yaml
sectors: {}
events: {}
regions: {}
entities:
  companies: {}
```

### 2. Run analytics:

```bash
go run ./cmd/korel-analytics \
  --input=your-data.jsonl \
  --stoplist=configs/mydomain/stoplist.yaml \
  --dict=configs/mydomain/tokens.dict \
  --taxonomy=configs/mydomain/taxonomies.yaml
```

### 3. Review suggestions and update configs

Add suggested stopwords, multi-token phrases, and taxonomy categories based on your domain knowledge.

### 4. Ingest and query!

---

## 🆘 Troubleshooting

### "No results found"
- **Cause:** Query tokens all filtered as stopwords
- **Fix:** Check that your dictionary includes multi-token phrases like "open source"

### "All flags required"
- **Cause:** Missing required flags
- **Fix:** All tools require explicit paths (no defaults)

### "PMI scores all 0.0"
- **Cause:** Need more documents for reliable PMI
- **Fix:** Use at least 100 documents; 50 is minimum for testing

### Analytics returns no stopword_candidates
- **Cause:** Corpus too small or entropy too low
- **Fix:** Review `high_df_tokens` manually and add to stoplist

---

## 📚 Next Steps

1. **Explore the data** - Query different terms and see PMI relationships
2. **Scale up** - Download 200-500 documents for better PMI signals
3. **Autotune** - After ingestion, run autotune for ongoing improvements (coming soon)
4. **Add symbolic rules** - Create `.rules` files for domain reasoning
5. **Integrate with agents** - See `docs/AGENT_INTEGRATION.md`

---

## 💡 Key Concepts

**Bootstrap workflow:**
```
Raw Data → Analytics → Suggestions → Seed Configs → Ingest → Query
```

**After initial ingest:**
```
Query → Results → Review → Autotune → Maintenance → Better Results
```

**Why this matters:**
- ✅ Start with ANY corpus in 2 minutes
- ✅ System learns domain-specific stopwords
- ✅ Explainable results (PMI scores, matched tokens)
- ✅ No black-box embeddings

---

Ready to start? Pick a downloader and go! 🎉
