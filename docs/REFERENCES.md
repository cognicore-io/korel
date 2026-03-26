# References & Related Work

This document tracks external projects, publications, and ideas that inform Korel's design.

---

## Vincent Granville — xLLM / BondingAI

Primary inspiration for Korel's statistical + symbolic approach to enterprise AI.

### xLLM 2.0 (March 2026)

- **Blog post:** https://mltechniques.com/2026/03/05/xllm-version-2-0-github-repository-with-innovative-ai-agents
- **GitHub:** https://github.com/VincentGranville/xLLM (Python, 77 commits)
- **Company:** https://bondingai.io/
- **eBook:** "No-Blackbox, Secure, Efficient AI and xLLM Solutions" — https://mltechniques.com/product/no-blackbox-secure-efficient-ai-and-llm-solutions/
- **eBook ToC:** https://mltblog.com/4aedKl2
- **Older repo (xLLM 1.0 / LLMs):** https://github.com/VincentGranville/Large-Language-Models

### xLLM 2.0 — 19 Base Components

| # | xLLM Component | Korel Equivalent | Status |
|---|----------------|------------------|--------|
| 1 | Font Intelligence (PDF parsing) | — | Not planned (out of scope) |
| 2 | Sorted N-grams (attention via ordering) | Multi-token recognition (`ingest/multitoken.go`) | Implemented (greedy longest-match) |
| 3 | Multitoken types (synonyms, contextual) | Lexicon package (`pkg/korel/lexicon`) | Implemented |
| 4 | Nested hashes (in-memory DB) | Memstore (`store/memstore`) + SQLite | Implemented |
| 5 | Multitoken distillation | — | Not yet |
| 6 | Relevancy scores | Hybrid scoring (`pkg/korel/rank`) | Implemented |
| 7 | Trustworthiness scores | Authority component in scoring (eta weight) | Partial |
| 8 | PMI metric (prompt suggestion) | PMI co-occurrence (`pkg/korel/pmi`) | Implemented (retrieval, not prompt suggestion) |
| 9 | Hierarchical chunking & multi-index | Feature Idea #1 (KG chunking) | Planned |
| 10 | Multimodal processing | — | Not planned |
| 11 | xLLM file format (JSON-like) | JSONL corpus format | Implemented |
| 12 | Advanced search (exact, broad, negative, recency, category) | Query parser + taxonomy filtering | Implemented |
| 13 | Advanced UI (explainable, real-time tuning) | CLI cards + score breakdowns | Partial (CLI only) |
| 14 | Real-time fine-tuning | Configurable score weights | Implemented |
| 15 | Proprietary stemmer/unstemmer | Self-adjusting stoplist + lexicon | Implemented |
| 16 | Evaluation metrics (exhaustivity) | — | Not yet |
| 17 | Synthetic prompt generation | — | Not yet |
| 18 | Auto-tagging & auto-indexing | AutoTune (`pkg/korel/autotune`) | Implemented |
| 19 | Variable-length embeddings | PMI-based similarity (no embeddings) | Alternative approach |

### xLLM 2.0 — AI Agents

| Agent | Description | Korel Opportunity |
|-------|-------------|-------------------|
| Anomaly Detection | Cybersecurity / fraud litigation; animated map visualization | PMI drift detection between time windows; anomalous co-occurrence shifts |
| NoGAN Data Synthesis | Tabular enterprise data (fraud, medical); proprietary non-GAN approach | Synthetic corpus generation for testing; bootstrap from small seed data |
| ECG Medical Agent | Electrocardiogram pattern detection; high compression | Domain-specific pipeline example; time-series token patterns |

### xLLM 2.0 — DNN Claims

- 96% next-token prediction on NVIDIA corporate corpus, built from scratch (no PyTorch/TF)
- Multitokens reduce vocabulary 10,000x vs generic models
- "Universal functions" with explainable parameters, no activation functions
- Distillation-resistant watermarking for IP protection
- "Benign overfitting" as a feature

### Key Shared Principles

Both xLLM and Korel share:
- No blackbox — every decision is auditable
- No GPU required — CPU-sufficient, on-premises
- No hallucinations — retrieval only, no generation
- Statistical foundations over neural architectures
- PMI as a core signal
- Multi-token recognition as semantic units
- Self-tuning / auto-indexing capabilities
- Enterprise-first (security, compliance, IP control)

### Where Korel Diverges

- **Language:** Go vs Python — Korel targets deployment as a compiled binary/library
- **Symbolic reasoning:** Korel adds a rule engine (inference/simple) absent from xLLM
- **Proof chains:** Korel explains not just scores but logical derivation paths
- **No DNN component:** Korel is purely statistical + symbolic, no neural network layer
- **Open architecture:** Swappable inference engine interface; xLLM is more monolithic

---

## Ideas Sparked by xLLM 2.0

### 1. PMI Anomaly Detection

Use PMI drift between time windows to flag anomalous shifts in term co-occurrence.
A token pair whose PMI jumps or drops significantly between week N and week N+1
could signal emerging threats, trending topics, or data quality issues.

Potential implementation: `pkg/korel/signals/anomaly.go` — compare PMI snapshots
across time partitions stored in SQLite.

### 2. Evaluation Metrics (Exhaustivity)

xLLM's component #16 measures how exhaustively results cover the query intent.
Korel could add a coverage score: what fraction of query tokens (and their expansions)
are represented in the returned card set.

### 3. Synthetic Prompt Generation

xLLM's component #17 generates test prompts. Korel could use high-PMI token clusters
to auto-generate evaluation queries, enabling automated quality benchmarks after
AutoTune rounds.

### 4. Trustworthiness Scores

Extend Korel's authority signal beyond link count. Consider source age, update
frequency, cross-reference density, and domain reputation as inputs to a
composite trust score per document.

---

## Classical Foundations

### Statistical NLP (1990s-2000s)

- IBM alignment models and n-gram language models
- Smoothing techniques: Kneser-Ney, Good-Turing, Witten-Bell
- PMI and pointwise mutual information (Church & Hanks, 1990)
- "Web as corpus" approaches (Kilgarriff & Grefenstette, 2003)
- BM25 ranking (Robertson et al., 1995)

### Symbolic AI (1980s-1990s)

- Expert systems and production rule engines
- Prolog and logic programming
- Taxonomic reasoning and ontologies
- Frame-based knowledge representation

### Modern Hybrid Systems

- Vespa (hybrid search: BM25 + neural)
- Elasticsearch (statistical retrieval at scale)
- Neo4j / Dgraph (explicit knowledge graphs)
- Weaviate (vector + keyword search)

---

## Author

Vincent Granville, PhD — Cofounder/CAIO at BondingAI.io. Former Cambridge post-doc,
20+ years at CNET, NBC, Visa, Wells Fargo, Microsoft, eBay. Founded Data Science
Central (acquired by TechTarget). Wiley/Elsevier author. Seattle-based.

LinkedIn: https://www.linkedin.com/in/vincentg/
Email: vincent@bondingai.io
