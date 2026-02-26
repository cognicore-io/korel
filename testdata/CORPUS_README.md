# Korel Test Corpora

Two test corpora for validating the Korel knowledge retrieval system.

## Corpus 1: Hacker News Stories

**Source:** Top stories from news.ycombinator.com
**File:** `hn/docs.jsonl`
**Size:** 50 documents
**Date Created:** 2025-11-10

### Category Distribution
- tech: 37 (74%)
- opensource: 5 (10%)
- ai: 5 (10%)
- web: 3 (6%)
- security: 2 (4%)
- programming: 1 (2%)

### Content
Current tech news, startups, programming discussions, open source projects.

### Example Document
```json
{
  "url": "https://news.ycombinator.com/item?id=45838592",
  "title": "Show HN: What Is Hacker News Working On?",
  "published_at": "2025-11-06T19:31:06+01:00",
  "outlet": "news.ycombinator.com",
  "text": "Show HN: What Is Hacker News Working On?...",
  "source_cats": ["security"]
}
```

## Corpus 2: arXiv AI Research Papers

**Source:** arXiv.org cs.AI category
**File:** `arxiv/docs.jsonl`
**Size:** 50 documents
**Date Created:** 2025-11-10

### Category Distribution
- ai: 50 (100%)
- cs: 22 (44%)
- machine-learning: 18 (36%)
- computer-vision: 14 (28%)
- nlp: 6 (12%)
- software-engineering: 4 (8%)
- statistics: 3 (6%)
- others: <2% each

### Content
Recent AI research papers including:
- Video understanding and temporal search
- Large language models
- Computer vision and multimodal AI
- Machine learning theory and applications

### Example Document
```json
{
  "url": "http://arxiv.org/abs/2511.05489v1",
  "title": "TimeSearch-R: Adaptive Temporal Search for Long-Form Video Understanding...",
  "published_at": "2025-11-07T18:58:25Z",
  "outlet": "arxiv.org",
  "text": "TimeSearch-R: Adaptive Temporal Search...",
  "source_cats": ["computer-vision", "ai"]
}
```

## Usage

### Download New Corpora

```bash
# Hacker News (default: 100 stories)
go run ./cmd/download-hn [count]

# Examples:
go run ./cmd/download-hn 50      # Download 50 stories
go run ./cmd/download-hn 200     # Download 200 stories

# arXiv AI papers (default: 200 papers)
go run ./cmd/download-arxiv cs.AI [count]

# Examples:
go run ./cmd/download-arxiv cs.AI 50    # 50 AI papers
go run ./cmd/download-arxiv cs.CL 100   # 100 NLP papers
go run ./cmd/download-arxiv cs.LG 100   # 100 ML papers
go run ./cmd/download-arxiv econ.EM 50  # 50 Economics papers
go run ./cmd/download-arxiv q-fin 50    # 50 Finance papers
```

**Output locations:**
- HN stories: `testdata/hn/docs.jsonl`
- arXiv papers: `testdata/arxiv/docs.jsonl`

### Ingest into Korel

**Step 1: Ingest HN corpus**
```bash
go run ./cmd/rss-indexer \
  -db ./data/hn.db \
  -data testdata/hn/docs.jsonl \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**Step 2: Ingest arXiv corpus**
```bash
go run ./cmd/rss-indexer \
  -db ./data/arxiv.db \
  -data testdata/arxiv/docs.jsonl \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**What happens during ingestion:**
1. Opens/creates SQLite database
2. Loads configuration (stoplist, dictionary, taxonomy)
3. For each document:
   - Tokenizes text (splits into words, removes stopwords)
   - Recognizes multi-token phrases ("machine learning" → single token)
   - Assigns categories based on keyword matching
   - Extracts entities (if taxonomy defines them)
4. Builds PMI co-occurrence statistics
   - Tracks which tokens appear together
   - Calculates PMI scores for token pairs
   - Stores document frequency (DF) for each token
5. Stores documents and metadata in SQLite

**Performance:**
- 50 documents: ~5-10 seconds
- 100 documents: ~15-20 seconds
- 500 documents: ~1-2 minutes

### Query the Indexed Data

**Query HN corpus:**
```bash
go run ./cmd/chat-cli \
  -db ./data/hn.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**Try these queries on HN:**
- `open source`
- `startup funding`
- `security vulnerability`
- `programming language`
- `web browser`
- `ai project`

**Query arXiv corpus:**
```bash
go run ./cmd/chat-cli \
  -db ./data/arxiv.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml
```

**Try these queries on arXiv:**
- `machine learning`
- `neural network`
- `computer vision`
- `natural language processing`
- `reinforcement learning`
- `deep learning`

**One-shot queries (non-interactive mode):**
```bash
# Query HN corpus directly
go run ./cmd/chat-cli \
  -db ./data/hn.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -query "open source"

# Query arXiv corpus with custom topK
go run ./cmd/chat-cli \
  -db ./data/arxiv.db \
  -stoplist testdata/hn/stoplist.yaml \
  -dict testdata/hn/tokens.dict \
  -taxonomy testdata/hn/taxonomies.yaml \
  -query "machine learning" \
  -topk 5
```

### Verify Ingestion

**Check document count:**
```bash
sqlite3 data/hn.db "SELECT COUNT(*) FROM docs;"
sqlite3 data/arxiv.db "SELECT COUNT(*) FROM docs;"
```

**Check token statistics:**
```bash
# Number of unique tokens
sqlite3 data/hn.db "SELECT COUNT(*) FROM token_df;"

# Number of token pairs (PMI)
sqlite3 data/hn.db "SELECT COUNT(*) FROM token_pairs;"

# Top 10 most frequent tokens
sqlite3 data/hn.db "SELECT token, df FROM token_df ORDER BY df DESC LIMIT 10;"
```

**Check database size:**
```bash
ls -lh data/*.db
# Should be >100KB for 50 documents, grows with corpus size
```

## Corpus Characteristics

| Characteristic | HN Corpus | arXiv Corpus |
|----------------|-----------|--------------|
| Domain | Tech news/startups | AI research |
| Text length | Short-medium (1-2 paragraphs) | Long (abstracts + authors) |
| Categories | 6 types | 10+ types |
| Temporal | Current (Nov 2025) | Recent (Nov 2025) |
| Language | Informal/conversational | Academic/technical |
| Source diversity | High (many sites) | Single source (arXiv) |

## Test Scenarios

These corpora enable testing:

1. **Multi-domain retrieval** - Tech news vs academic papers
2. **Category overlap** - "ai" appears in both but different contexts
3. **Text complexity** - Short vs long documents
4. **Temporal search** - Recent content from similar time period
5. **Entity extraction** - Companies (HN) vs authors/institutions (arXiv)
6. **PMI co-occurrence** - Different term relationships per domain

## Data Quality

- All documents have valid URLs, titles, timestamps
- Categories assigned by keyword matching (heuristic)
- HTML stripped from HN stories
- arXiv abstracts cleaned of extra whitespace
- No duplicates (verified by unique URLs)
