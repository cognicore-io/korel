# Bootstrapping a New Corpus from Scratch

## Overview

When starting with a completely new corpus/domain, Korel can automatically generate initial configurations. This document explains the automated bootstrap workflow with optional LLM review.

> ℹ️ **NOTE:** Bootstrap is fully configurable for different corpora sizes, languages, and domains. See the **Advanced Configuration** section below for flags that control stoplist seeding, taxonomy clustering thresholds, and domain-specific blacklists.

---

## Quick Start: Automated Bootstrap

### Without LLM (Statistical Analysis Only)

```bash
go run ./cmd/bootstrap \
  --input=testdata/hn/docs.jsonl \
  --domain=hn \
  --output=configs/hn/ \
  --iterations=2 \
  --pair-min-support=10 \
  --pair-min-pmi=0.5
```

**Generates:**
- `stoplist.yaml` - High-DF tokens discovered automatically (iteratively refined)
- `synonyms.yaml` - Synonym candidates from c-token analysis (requires manual review)
- `tokens.dict` - High-PMI phrase pairs (bigram frequency × document PMI)
- `taxonomies.yaml` - Category structure from clustering
- `bootstrap-report.json` - Statistical analysis with iteration log

**Example output:**
```
2025/11/11 06:13:32 Loaded 125 base stopwords from configs/stopwords-en.yaml
2025/11/11 06:13:32 === Iteration 1/2: analyzing with 125 stopwords ===
2025/11/11 06:13:32 Discovered 15 new stopwords: [shown, however, using, ...]
2025/11/11 06:13:33 === Iteration 2/2: analyzing with 140 stopwords ===
2025/11/11 06:13:33 Converged! No new stopwords discovered in iteration 2.
2025/11/11 06:13:33 Bootstrap complete after 2 iterations
Bootstrap configs written to configs/hn/
```

### With LLM Review (Recommended)

```bash
go run ./cmd/bootstrap \
  --input=testdata/hn/docs.jsonl \
  --output=configs/hn/ \
  --iterations=2 \
  --llm-config=configs/bootstrap-llm.yaml
```

**Generates everything above PLUS:**
- `bootstrap-reviewed.json` - After LLM validation (source of truth)
- `bootstrap-llm.log` - LLM prompts and responses

**LLM validates:**
- ✅ Stopwords (filters out technical terms)
- ✅ Multi-token phrases (approves meaningful combinations)
- ✅ Taxonomy categories (generates coherent structure)
- ✅ Entities (identifies companies/products/technologies)

---

## Complete Workflow

```
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Bootstrap (1 command, ~30 seconds)                  │
├─────────────────────────────────────────────────────────────┤
│ go run ./cmd/bootstrap --input=data.jsonl --output=configs/ │
│                                                               │
│ → Analyzes corpus (DF, PMI, clustering)                     │
│ → Generates configs automatically                            │
│ → Optional: LLM reviews and refines                          │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Review (optional, manual)                           │
├─────────────────────────────────────────────────────────────┤
│ vim configs/bootstrap-reviewed.json                         │
│                                                               │
│ → Edit single JSON file (not 3+ YAML files)                 │
│ → Re-apply: go run ./cmd/bootstrap --apply=reviewed.json    │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 3: Ingest (~10 seconds for 50 docs)                    │
├─────────────────────────────────────────────────────────────┤
│ go run ./cmd/rss-indexer --db=data.db --data=data.jsonl ... │
│                                                               │
│ → Uses generated configs                                     │
│ → Builds PMI statistics                                      │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 4: Query (instant)                                      │
├─────────────────────────────────────────────────────────────┤
│ go run ./cmd/chat-cli --db=data.db --query="search term"    │
│                                                               │
│ → Retrieves with explainable results                        │
└─────────────────────────────────────────────────────────────┘
```

---

## How Bootstrap Works

### Iterative Refinement Process

Bootstrap uses **iterative analysis** to progressively improve configuration quality:

```
Dataset: arxiv-iterative
Iteration 1: base stopwords (125) → discover new (15) → total (140)
Iteration 2: base stopwords (140) → discover new (0)  → CONVERGED ✓

Dataset: hn-sample
Iteration 1: base stopwords (125) → discover new (15) → total (140)
Iteration 2: base stopwords (140) → discover new (3)  → total (143)
Iteration 3: base stopwords (143) → discover new (0)  → CONVERGED ✓
```

**Why iterative?**
- First pass may miss stopwords that only become visible after removing other noise
- Example: "can be" bigram only disappears after BOTH "can" AND "be" are filtered
- Typically converges in 2-3 iterations for 500+ documents
- Use `--iterations=3` flag (default: 2)
- Every iteration re-learns stopwords, multi-token phrases, and taxonomy seeds directly from the observed corpus statistics; no hard-coded vocabulary is baked into the binary.

### Phase 1: Statistical Analysis with Dual Tracking

**Bootstrap performs in-memory analysis with TWO types of pair counting:**

#### 1. Bigram Tracking (Adjacent Pairs)
Counts only consecutive token pairs in sequence:
- Text: "deep learning models use deep neural networks"
- Bigrams: `(deep,learning)`, `(learning,models)`, `(models,use)`, `(use,deep)`, `(deep,neural)`, `(neural,networks)`
- Purpose: **Phrase discovery** - finds fixed collocations

#### 2. Document-level Co-occurrence (All Pairs)
Counts all unique token pairs within each document:
- Same text: All combinations of unique tokens
- Pairs: `(deep,learning)`, `(deep,models)`, `(deep,use)`, `(deep,neural)`, ... (15 pairs total)
- Purpose: **Semantic filtering** - measures conceptual relatedness via PMI

#### Combined Scoring Formula

For phrase discovery, both signals are combined:

```go
phraseScore = bigramFrequency × documentPMI

// Filter criteria:
// 1. bigramFrequency >= 10  (appears adjacent at least 10 times)
// 2. documentPMI >= 0.5      (semantically related, not random)
```

**Concrete example (real `arxiv-iterative` run):**

- The bigram `(deep,learning)` appears adjacent **450** times across the corpus windows.
- The same tokens co-occur in **180** distinct documents, giving a document PMI of **0.59**.
- Combined score: `phraseScore = 450 × 0.59 = 265.5`.
- Result: the phrase is promoted into `tokens.dict`, while its components stay available for PMI counting elsewhere.

**Why this works:**

| Phrase | Bigram Freq | PMI | Score | Result |
|--------|-------------|-----|-------|--------|
| "machine learning" | 98 | 0.59 | 58 | ✅ Keep |
| "can be" | 90 | 0.03 | 2.7 | ❌ Filter (stopword) |
| "learning theory" | 3 | 1.8 | 5.4 | ❌ Filter (not fixed phrase) |

**Dual tracking recap:**
- Bigrams: `(deep,learning)` appears adjacent **450** times (evidence of a contiguous phrase).
- Document PMI: `"deep"` and `"learning"` co-occur in **180** documents with PMI **0.59**.
- Combined: `phraseScore = 450 × 0.59 = 265.5` → qualifies for `tokens.dict`.
- Stopword discovery: `"can be"` keeps popping up with low PMI, so it is queued as a stopword candidate for the next refinement pass.

**Steps:**

1. **Tokenization** - Process all documents with current stoplist
2. **Document Frequency** - Count how often each token appears
3. **Dual PMI Calculation** - Track bigrams AND document pairs (in-memory)
4. **Stopword Discovery** - Find high-DF/low-PMI candidates
5. **Clustering** - Group related terms for taxonomy
6. **Entity Extraction** - Identify capitalized terms (companies/products)

**Outputs:**
```json
{
  "stopword_candidates": [
    {"token": "https", "df_percent": 96, "entropy": 0, "score": 0.79}
  ],
  "high_pmi_pairs": [
    {"token1": "machine", "token2": "learning", "pmi": 2.8, "cooccur": 15}
  ],
  "frequent_terms": [
    {"token": "learning", "freq": 45, "contexts": ["machine", "deep", "neural"]}
  ],
  "entities": [
    {"token": "OpenAI", "freq": 8, "contexts": ["gpt", "chatgpt"]}
  ]
}
```

### Phase 2: LLM Review (Optional)

**4 LLM tasks automatically run:**

#### Task 1: Stopword Validation
**Reviews:** Top 20 high-DF candidates
**Prompt:** "Approve only generic terms (URL components, universal words, filler). Reject technical terms and proper nouns."
**Output:** `["https", "com", "source"]`

#### Task 2: Multi-Token Approval
**Reviews:** Top 50 high-PMI pairs
**Prompt:** "Identify meaningful phrases (technical terms, compound concepts, industry terms). Reject grammatical patterns."
**Output:** `[{"phrase": "machine learning", "category": "ai"}]`

#### Task 3: Taxonomy Generation
**Reviews:** Top 100 frequent terms with contexts
**Prompt:** "Create 5-10 categories with 10-20 keywords each. Use industry-standard names."
**Output:**
```json
{
  "sectors": {
    "ai": {"name": "AI & ML", "keywords": ["ai", "machine learning", ...]},
    "security": {"name": "Security", "keywords": ["vulnerability", ...]}
  }
}
```

#### Task 4: Entity Classification
**Reviews:** Top 50 capitalized terms
**Prompt:** "Classify as company, product, or technology."
**Output:** `{"companies": ["OpenAI"], "technologies": ["React"]}`

### Phase 3: Config Generation

**Bootstrap writes YAML files:**

**stoplist.yaml:**
```yaml
terms:
  - the      # Universal English
  - a
  - https    # From DF analysis
  - com      # LLM approved
  - source
```

**tokens.dict:**
```
# Format: phrase|display|category
# Phrases discovered via bigram frequency × document PMI
machine learning|machine learning|general
neural networks|neural networks|general
reinforcement learning|reinforcement learning|general
artificial intelligence|artificial intelligence|general
time series|time series|general
```

**taxonomies.yaml:**
```yaml
sectors:
  ai:
    - ai
    - machine learning
    - neural network
  security:
    - security
    - vulnerability
    - encryption
events: {}
regions: {}
entities:
  companies:
    - OpenAI
    - Google
  technologies:
    - React
    - PyTorch
```

**bootstrap-report.json:**
```json
{
  "generated_at": "2025-11-11T06:13:33Z",
  "total_docs": 500,
  "iterations_total": 2,
  "iterations": [
    {
      "iteration": 1,
      "stopwords_added": 15,
      "new_stopwords": ["shown", "however", "using", "provide", ...],
      "total_stopwords": 140
    },
    {
      "iteration": 2,
      "stopwords_added": 0,
      "new_stopwords": null,
      "total_stopwords": 140
    }
  ],
  "stopwords": ["the", "a", "and", ...],
  "pairs": [
    {
      "A": "machine",
      "B": "learning",
      "PMI": 0.59,
      "BigramFreq": 98,
      "Support": 180,
      "PhraseScore": 58.0
    }
  ],
  "high_df_tokens": [...],
  "taxonomy": {...}
}
```

**Key fields:**
- `iterations` - Tracks convergence: how many stopwords added per iteration
- `iterations_total` - Mirrors CLI `--iterations` to show how many loops actually ran
- `pairs[].BigramFreq` - How often tokens appear adjacent (phrase strength)
- `pairs[].PMI` - Document-level semantic relatedness (filters stopwords)
- `pairs[].PhraseScore` - Combined score (BigramFreq × PMI) for ranking

---

## LLM Configuration

**File: `configs/bootstrap-llm.yaml`**

### Endpoints (Choose One)

**Ollama (Local, Free):**
```yaml
endpoint:
  base_url: "http://localhost:11434/v1"
  model: "llama2"
  api_key: ""
  temperature: 0.2
  max_tokens: 2000
```

**OpenAI:**
```yaml
endpoint:
  base_url: "https://api.openai.com/v1"
  model: "gpt-4"
  api_key_env: "OPENAI_API_KEY"
  temperature: 0.2
  max_tokens: 2000
```

**Azure:**
```yaml
endpoint:
  base_url: "https://YOUR_RESOURCE.openai.azure.com"
  model: "gpt-4"
  api_key_env: "AZURE_OPENAI_KEY"
  azure_deployment: "gpt-4-deployment"
  temperature: 0.2
```

### Task Configuration

```yaml
tasks:
  stopwords:
    enabled: true
    review_limit: 20  # Review top 20, auto-approve rest if DF > 90%

  multi_tokens:
    enabled: true
    min_pmi: 2.0      # Only review pairs with PMI >= 2.0
    review_limit: 50

  taxonomy:
    enabled: true
    top_terms: 100    # Analyze top 100 terms

  entities:
    enabled: true
    review_limit: 50

validation:
  stopwords:
    auto_approve_df_threshold: 95.0  # Auto-approve if DF > 95%
    reject_df_threshold: 5.0          # Reject if DF < 5%
  multi_tokens:
    min_pmi: 1.5
    min_cooccur: 3
  taxonomy:
    min_keywords_per_category: 5
    max_categories: 15
```

### Customizing Prompts

```yaml
tasks:
  stopwords:
    system_prompt: |
      You are a linguistic expert reviewing stopword candidates.

    user_prompt_template: |
      Review these candidates for a {{DOMAIN}} corpus:
      {{CANDIDATES}}

      Return JSON array: ["approved1", "approved2"]
```

**Template variables:**
- `{{DOMAIN}}` - Corpus description
- `{{CANDIDATES}}` - Formatted list
- `{{PAIRS}}` - Token pairs
- `{{TERMS}}` - Frequent terms

---

## File Versioning & Audit Trail

**Bootstrap keeps both versions:**

```
configs/hn/
├── bootstrap-raw.json          # Statistical analysis (original)
├── bootstrap-reviewed.json     # After LLM review (source of truth)
├── bootstrap-llm.yaml          # LLM config used
├── stoplist.yaml              # Generated from reviewed
├── tokens.dict                # Generated from reviewed
└── taxonomies.yaml            # Generated from reviewed
```

**Benefits:**
- ✅ Diff raw vs reviewed to see LLM changes
- ✅ Re-run with different LLM config
- ✅ Manually edit reviewed JSON and re-apply
- ✅ Full audit trail

**Example: Re-apply after manual edit:**
```bash
# 1. Edit LLM-reviewed output
vim configs/hn/bootstrap-reviewed.json

# 2. Regenerate YAML files
go run ./cmd/bootstrap \
  --apply=configs/hn/bootstrap-reviewed.json \
  --output=configs/hn/
```

---

## Bootstrap Modes

### Mode 1: From Scratch (Nothing Exists)

```bash
# Creates minimal bootstrap configs internally
go run ./cmd/bootstrap \
  --input=new-corpus.jsonl \
  --output=configs/newdomain/
```

**Uses internal minimal configs:**
- Stoplist: Just 12 universal stopwords (the, a, and...)
- Dict: Empty
- Taxonomy: Empty

### Mode 2: With Existing Minimal Configs

```bash
go run ./cmd/bootstrap \
  --input=data.jsonl \
  --output=configs/domain/ \
  --stoplist=testdata/bootstrap/stoplist.yaml \
  --dict=testdata/bootstrap/tokens.dict \
  --taxonomy=testdata/bootstrap/taxonomies.yaml
```

**Uses provided configs as starting point.**

### Mode 3: Re-apply After Manual Edit

```bash
# Bootstrap already ran, now re-apply edits
go run ./cmd/bootstrap \
  --apply=configs/hn/bootstrap-reviewed.json \
  --output=configs/hn/
```

**Skips analysis, just regenerates YAML from JSON.**

---

## Bootstrap Detection Strategies

### Stopword Detection

**Relaxed thresholds when PMI unavailable:**
- DF > 60% (instead of 80%)
- Entropy > 0.4 (instead of 0.8)
- PMI requirement dropped (PMI=0 during bootstrap)

**Fallback: DF-only mode**
- If entropy data sparse, use DF threshold only

**Validation after LLM:**
- Auto-approve if DF > 95%
- Reject if DF < 5%
- Trust LLM for middle range

### Synonym Discovery

**C-token analysis for synonym candidates:**
- Uses skip-gram co-occurrence (default window: 5 tokens)
- PMI > 1.5: Strong contextual relationship
- Co-occur > 3: Minimum frequency for reliability
- Clustering by shared context

**Algorithm:**
1. Track token pairs within skip-gram windows
2. Calculate PMI for contextual strength
3. Build graph of high-PMI relationships
4. Find connected components (mutually related tokens)
5. Output as synonym candidate groups

**Output requires manual review:**
- ✅ Keep true synonyms: `game/games/gaming`
- ❌ Remove collocations: `machine/learning` (related, not synonyms)
- ❌ Remove spurious associations from small corpus

**Threshold tuning:**
- Higher `--synonym-min-pmi`: Fewer, higher-quality candidates
- Lower `--synonym-min-pmi`: More candidates, more false positives
- Increase `--synonym-min-support` for larger corpora (5-10)

**Best for:**
- Morphological variants (analyze/analysis/analytical)
- Abbreviations and expansions (ML/machine learning)
- Domain-specific terminology normalization

**See also:** [docs/LEXICON.md](LEXICON.md) for complete lexicon system documentation

### Multi-Token Discovery

**High PMI pairs indicate meaningful phrases:**
- PMI > 2.0: Strong association
- Co-occur > 3: Minimum frequency
- Sent to LLM for semantic validation

**LLM filters out:**
- Grammatical patterns ("very good")
- Generic combinations ("big company")
- Spurious correlations

### Taxonomy Generation

**Clustering approach:**
1. Find frequent terms (top 100)
2. Analyze their contexts (co-occurring words)
3. Group related terms
4. LLM generates category names and structure

**LLM ensures:**
- Clear category names
- Mutually exclusive categories (where possible)
- Industry-standard terminology
- 5-10 top-level categories

### Entity Recognition

**Heuristic extraction:**
1. Find capitalized terms (not at sentence start)
2. Count frequency
3. Analyze contexts

**LLM classifies:**
- Company: Organizations (OpenAI, Google)
- Product: Software/services (ChatGPT, Chrome)
- Technology: Frameworks/languages (React, Python)
- Skip: Acronyms, non-entities

---

## Cost & Performance

### Statistical Analysis (No LLM)
- **Time:** ~10-30 seconds for 50-500 docs
- **Cost:** Free
- **Output:** Statistical suggestions

### With LLM Review

**OpenAI GPT-4:**
- 50 docs: ~$0.08
- 500 docs: ~$0.50
- **Breakdown:**
  - Stopwords: ~500 tokens → $0.01
  - Multi-tokens: ~1000 tokens → $0.02
  - Taxonomy: ~2000 tokens → $0.04
  - Entities: ~500 tokens → $0.01

**Ollama (Local):**
- **Cost:** Free
- **Time:** ~2-5 minutes (depends on hardware)
- **Quality:** Slightly lower but acceptable

**Azure:**
- Similar to OpenAI pricing
- Use existing enterprise agreements

---

## Troubleshooting

### No Stopword Candidates Returned

**Cause:** Corpus too small or entropy too low
**Fix:**
- Review `high_df_tokens` in JSON manually
- Use `--llm-config` for validation
- Need at least 30-50 documents

### Multi-Token Pairs Look Random

**Cause:** Corpus too small for reliable PMI
**Fix:**
- Need 100+ documents for good PMI
- LLM review filters spurious pairs
- Manual review in JSON

### Taxonomy Too Broad/Narrow

**Cause:** LLM configuration
**Fix:**
- Adjust `top_terms` in config (increase for more categories)
- Edit `max_categories` threshold
- Provide domain context in prompts

### LLM Review Failing

**Cause:** API key, rate limits, or format issues
**Fix:**
- Check `bootstrap-llm.log` for errors
- Verify endpoint and API key
- Try Ollama locally for testing

---

## Best Practices

### For Best Results

1. **Corpus Size:**
   - Minimum: 50 documents (for testing)
   - Recommended: 200+ documents (for reliable statistics)
   - Production: 1000+ documents

2. **Domain Context:**
   - Customize LLM prompts with domain description
   - Add few-shot examples to prompts
   - Use domain-specific terminology in prompts

3. **Iterative Refinement:**
   - Run bootstrap
   - Review output
   - Adjust LLM config
   - Re-run with tweaked prompts

4. **Manual Review:**
   - Always review `bootstrap-reviewed.json`
   - Check for missed technical terms in stopwords
   - Verify multi-token phrases make sense
   - Ensure taxonomy categories are coherent

5. **Version Control:**
   - Commit all bootstrap outputs
   - Track changes to LLM config
   - Document manual edits in reviewed JSON

---

## Example: Hacker News Domain

```bash
# 1. Download data
go run ./cmd/download-hn 200

# 2. Bootstrap with LLM review
go run ./cmd/bootstrap \
  --input=testdata/hn/docs.jsonl \
  --output=configs/hn/ \
  --llm-config=configs/bootstrap-llm.yaml

# Output:
# ✓ Analyzed 200 documents
# ✓ Found 45 stopword candidates
# ✓ Discovered 78 multi-token phrases
# ✓ Generated 8 taxonomy categories
# ✓ Identified 23 entities
# ✓ LLM review: 32 stopwords approved
# ✓ LLM review: 45 phrases approved
# ✓ Configs written to configs/hn/

# 3. Review (optional)
cat configs/hn/bootstrap-reviewed.json

# 4. Ingest
go run ./cmd/rss-indexer \
  --db=./data/hn.db \
  --data=testdata/hn/docs.jsonl \
  --stoplist=configs/hn/stoplist.yaml \
  --dict=configs/hn/tokens.dict \
  --taxonomy=configs/hn/taxonomies.yaml

# 5. Query
go run ./cmd/chat-cli \
  --db=./data/hn.db \
  --stoplist=configs/hn/stoplist.yaml \
  --dict=configs/hn/tokens.dict \
  --taxonomy=configs/hn/taxonomies.yaml \
  --query="open source"
```

---

## Advanced Configuration

Bootstrap provides extensive configurability for different corpus sizes, languages, and domains.

### Stoplist Control

**Cold Start (No Base Stoplist)**
```bash
# True language-agnostic cold start - discover all stopwords from corpus
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --domain=mydomain \
  --no-base-stoplist \
  --iterations=3 \  # More iterations recommended without base
  --output=configs/
```

**Custom Base Stoplist**
```bash
# For non-English corpora
cat > configs/stopwords-de.yaml << 'EOF'
terms: [der, die, das, den, dem, des, ...]
EOF

go run ./cmd/bootstrap \
  --input=german-corpus.jsonl \
  --base-stoplist=configs/stopwords-de.yaml \
  --output=configs/de/
```

### Taxonomy Control

**Clustering Thresholds**
```bash
# Adjust for small corpora (<100 docs)
go run ./cmd/bootstrap \
  --input=small-corpus.jsonl \
  --taxonomy-min-support=1 \    # Default: 3
  --taxonomy-min-pmi=0.3 \       # Default: 0.8
  --output=configs/
```

**Domain-Specific Blacklist**
```bash
# Exclude domain-specific generic terms
cat > configs/finance-blacklist.yaml << 'EOF'
terms:
  - market
  - stock
  - trading
  - investor
  # These might be signal in medical domain, noise in finance
EOF

go run ./cmd/bootstrap \
  --input=finance-corpus.jsonl \
  --taxonomy-blacklist=configs/finance-blacklist.yaml \
  --output=configs/finance/
```

**Fail Fast on Empty Taxonomy**
```bash
# Require LLM taxonomy if clustering fails (no silent fallbacks)
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --require-taxonomy-llm \
  --taxonomy-llm-base=http://localhost:11434/v1 \
  --taxonomy-llm-model=llama2 \
  --output=configs/
```

Without `--require-taxonomy-llm`, bootstrap falls back to high-frequency keywords if clustering produces no results.

### Multi-Token Phrase Control

```bash
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --pair-min-support=5 \     # Default: 3 (min docs containing pair)
  --pair-min-pmi=1.5 \       # Default: 1.0 (semantic strength threshold)
  --pair-limit=20 \          # Default: 10 (max phrases to extract)
  --output=configs/
```

### Complete Flag Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | *required* | Path to JSONL corpus |
| `--domain` | *required* | Domain identifier |
| `--output` | *required* | Output directory |
| `--base-stoplist` | `configs/stopwords-en.yaml` | Base stopword list |
| `--no-base-stoplist` | `false` | Start from empty stoplist (cold start) |
| `--iterations` | `2` | Max iterative refinement iterations |
| `--stop-limit` | `25` | Stopword suggestions per iteration |
| `--pair-min-support` | `3` | Min support for multi-token pairs |
| `--pair-min-pmi` | `1.0` | Min PMI for multi-token pairs |
| `--pair-limit` | `10` | Max multi-token phrases |
| `--synonym-limit` | `20` | Max synonym candidates to suggest |
| `--synonym-min-support` | `3` | Min co-occurrence for synonym pairs |
| `--synonym-min-pmi` | `1.5` | Min PMI for synonym candidates |
| `--taxonomy-min-support` | `3` | Min support for clustering |
| `--taxonomy-min-pmi` | `0.8` | Min PMI for clustering |
| `--taxonomy-blacklist` | *(none)* | YAML file with generic terms to exclude |
| `--taxonomy-limit` | `150` | Max keywords per category |
| `--require-taxonomy-llm` | `false` | Fail if clustering fails (no fallback) |
| `--llm-base` | *(none)* | LLM base URL for stopword review |
| `--llm-model` | *(none)* | LLM model for stopword review |
| `--taxonomy-llm-base` | *(none)* | LLM base URL for taxonomy generation |
| `--taxonomy-llm-model` | *(none)* | LLM model for taxonomy generation |

### Use Case Examples

**Non-English Corpus (German)**
```bash
go run ./cmd/bootstrap \
  --input=german-papers.jsonl \
  --base-stoplist=configs/stopwords-de.yaml \
  --taxonomy-blacklist=configs/german-generics.yaml \
  --domain=de-papers \
  --output=configs/de/
```

**Small Corpus (<100 docs) with LLM**
```bash
go run ./cmd/bootstrap \
  --input=small-corpus.jsonl \
  --taxonomy-min-support=1 \
  --taxonomy-min-pmi=0.3 \
  --taxonomy-llm-base=http://localhost:11434/v1 \
  --taxonomy-llm-model=llama2 \
  --iterations=1 \
  --output=configs/
```

**Finance Domain (Custom Blacklist)**
```bash
go run ./cmd/bootstrap \
  --input=finance-news.jsonl \
  --taxonomy-blacklist=configs/finance-blacklist.yaml \
  --domain=finance \
  --output=configs/finance/
```

**Production (Strict Mode - No Silent Fallbacks)**
```bash
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --require-taxonomy-llm \
  --taxonomy-llm-base=https://api.openai.com/v1 \
  --taxonomy-llm-model=gpt-4 \
  --taxonomy-llm-api-key=$OPENAI_API_KEY \
  --domain=prod \
  --output=configs/prod/
```

---

## Implementation Status & Limitations

### What Works Today ✅

| Feature | Status |
|---------|--------|
| Bigram discovery with dual PMI tracking | ✅ Fully implemented |
| PMI-based stopword discovery (PMIMax) | ✅ Implemented (2025-11-11) |
| Iterative refinement (`--iterations`) | ✅ Implemented |
| Statistical taxonomy clustering | ✅ Implemented |
| LLM review for all components | ✅ Implemented |
| Text normalization (URLs, authors) | ✅ Implemented |

### Design Decisions & Trade-offs

**1. English Default Base Stoplist**
- **Default:** Loads ~125 English stopwords from `configs/stopwords-en.yaml`
- **Rationale:** Provides immediate high-quality results for English corpora (most common case)
- **Override:** Use `--base-stoplist` for other languages or `--no-base-stoplist` for true cold start

**2. Fallback Taxonomy Uses English/HN Generic Terms**
- **When:** Clustering fails (corpus too small or low PMI)
- **Behavior:** Falls back to high-frequency keyword extraction with default blacklist
- **Override:** Use `--taxonomy-blacklist` for domain-specific terms or `--require-taxonomy-llm` to fail fast

**3. Multi-Token Categories**
- **Behavior:** Assigns categories based on taxonomy cluster membership
- **Logic:** Prefers clusters where both tokens appear, falls back to first token's cluster
- **Empty category:** Means phrase doesn't match any cluster (intentional - shows unclassified)

**4. Statistical Clustering Requirements**
- **Works best:** 100+ documents with diverse vocabulary
- **Struggles:** <50 documents with high lexical overlap
- **Solution:** Lower `--taxonomy-min-support` and `--taxonomy-min-pmi` or use LLM taxonomy

---

## Future Features (Designed, Not Yet Implemented)

### N-gram Extension (Trigrams, 4-grams)

**Current:** Only bigrams ("machine learning")
**Planned:** Hierarchical n-gram building

```
Step 1: Discover bigrams ✅
  "machine learning", "neural networks"

Step 2: Extend to trigrams (only from validated bigrams)
  "large language" + "model" → "large language model"
  "graph neural" + "networks" → "graph neural networks"

Step 3: Extend to 4-grams (only from validated trigrams)
  "state of the art", "convolutional neural network layers"
```

**Key insight:** Don't track ALL n-grams - build hierarchically from validated shorter ones. Reduces search space by 10-100×.

### Auto-Generated Prolog Rules

**Current:** Manual rule creation
**Planned:** Auto-generate from bootstrap statistics

```prolog
% From bigrams
phrase(machine_learning, 2, 98).

% From document PMI
related_to(machine, learning, 0.59).
related_to(deep, learning, 0.89).

% From taxonomy
in_category(machine_learning, ai).

% From n-gram hierarchy
composite_of(large_language_model, [large, language, model]).
extends(large_language_model, language_model).

% Generic inference
transitive_related(X, Z, Score) :-
    related_to(X, Y, S1), related_to(Y, Z, S2),
    Score is S1 * S2 * 0.8.
```

**Integration with search:**
```bash
go run ./cmd/chat-cli --rules=configs/rules/auto.prolog --query="bert"
# Query expansion: bert → transformer → nlp → [related terms]
# Explainable reasoning with proof chains
```

**Implementation path:**
1. Use `pkg/korel/autotune/rules` for candidate generation
2. Use `pkg/korel/maintenance.RuleExporter` for serialization
3. Use `pkg/korel/inference/simple` engine for querying

---

## Summary

**Automated Bootstrap Process:**
```
1 command → 4 configs → Ready to ingest
```

**Strengths:**
- ✅ Works great for English corpora (100+ docs)
- ✅ Dual PMI tracking finds quality phrases
- ✅ Iterative refinement converges quickly
- ✅ Optional LLM review for all components

**Know Before Using:**
- English defaults (provide language-specific stoplist for others)
- Needs 100+ docs for clustering (use LLM for smaller)
- Taxonomy may be placeholder if clustering fails
- Always review generated configs before production

**The goal:** Start with ANY corpus in under 2 minutes, with clear workarounds for edge cases.
