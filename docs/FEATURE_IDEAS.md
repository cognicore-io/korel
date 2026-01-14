# Korel Feature Ideas (xLLM Alignment Roadmap)

This document captures forward-looking enhancements inspired by Vincent Granville’s xLLM (“LLM 2.0”) architecture. Each section describes the goal, proposed approach, and integration notes for Korel so it can serve as the statistical/symbolic core inside agentic RAG products while matching the explainability and determinism of enterprise xLLM deployments.

---

## 1. Knowledge-Graph Chunking & Hierarchical Cards

**Goal:** Move from document-level ingestion to fine-grained, context-rich chunks (parent/child relationships, breadcrumbs, tags) so cards mirror the structure of the underlying corpus and provide “infinite context windows.”

**Proposal:**
- Introduce a crawler/chunker module (e.g., `cmd/crawl-html`, `cmd/crawl-pdf`) that emits JSONL entries with `chunks: []`, each chunk carrying `id`, `parent`, `children`, `title`, `breadcrumbs`, `agents`, and `tags`.
- Extend `pkg/korel/ingest` to process chunk metadata alongside tokens/categories, storing results in a `chunks` table within SQLite (or a sidecar JSON for offline use).
- Update `pkg/korel/store` and `pkg/korel/cards` to rank and render cards at the chunk level, embedding parent context (e.g., “Section 2 ▸ Subsection a”). Provide API fields for `chunk_id`, `parent_path`, `has_table`, etc.
- Add configuration hooks to attach agent labels (e.g., “how-to”, “case study”) during chunking, enabling UI filters and agent routing.

**Integration Notes:** Hierarchical chunks unlock multi-index query routing (search within a section, a slide deck, or table) and enable future ACLs per chunk. They also align with xLLM’s “knowledge graph token” weighting when scoring results.

---

## 2. Synonym Maps, Multi-Tokens, and Contextual (c-)Tokens

**Goal:** Normalize domain-specific vocabulary (synonyms, acronyms, inflections) and exploit near-neighbor co-occurrence (c-tokens) to improve recall without embeddings.

**Proposal:**
- Add a `pkg/korel/lexicon` package that stores corpus-specific synonym/variant mappings (e.g., `analysis~variance ↔ anova`, `games ↔ gaming ↔ gamer`). Generate candidates automatically from bootstrap PMI pairs and high-entropy tokens; allow manual curation in `configs/<domain>/synonyms.yaml`.
- Enhance `ingest.Tokenizer` to consult the lexicon during normalization. Multi-token recognition already exists; extend it to handle acronym→expansion mappings and non-contiguous c-tokens.
- In `pkg/korel/analytics`, track skip-gram co-occurrences (tokens within a configurable window) and persist a `ctokens` map. At query time, if a token has sparse hits, look up correlated c-tokens to fetch additional candidates deterministically.
- Surface synonym/c-token traceability in score breakdowns so agents understand why a given card matched (“matched via synonym: gaming → game design”).

**Integration Notes:** This machinery fulfills Granville’s “g-tokens” (graph-derived) and “c-tokens” (contextual) concepts, reinforcing explainability: every expanded term is auditable and scored via PMI rather than hidden embeddings.

---

## 3. Router-Orchestrated Sub-LLMs

**Goal:** Deploy multiple specialized Korel instances (“sub-LLMs”) and route queries based on taxonomy, intent, or policy—mirroring xLLM’s router that selects category-specific engines.

**Proposal:**
- Package `korel` as a reusable service (gRPC/HTTP) exposing search + explain endpoints.
- Implement a router layer (`pkg/korel/router`) that:
  - Inspects query metadata (dominant taxonomy category, recency requirement, user profile/ACL).
  - Dispatches to one or more Korel instances dedicated to specific sectors or datasets (each with its own stoplist, multi-token dict, taxonomy).
  - Aggregates score breakdowns from sub-engines, applies cross-engine normalization, and returns merged cards.
- Provide configuration for agentic orchestration: define routes like `finance`, `legal`, `timeline`, each pointing to a Korel backend or to an external tool (e.g., math solver) for hybrid workflows.

**Integration Notes:** This unlocks enterprise scenarios where sensitive corpora stay siloed yet share a common query bus. It also makes it easy to attach Korel-based retrieval to AI agents (MCP tools, LangChain, LangGraph) with per-route grounding instructions.

---

## 4. Advanced Scoring Extensions

**Goal:** Bring Korel’s ranker even closer to xLLM’s “new PageRank” by incorporating knowledge-graph weights, multi-index positions, and agent signals.

**Proposal:**
- Extend `pkg/korel/rank` weights to include:
  - `LambdaKG`: boost when query tokens match knowledge-graph tokens (categories, tags, agents) attached to a chunk.
  - `MuStructure`: discount/boost based on chunk depth, presence of tables/figures, or agent type.
  - `NuCoverage`: penalize over-short cards or highlight cards covering multiple subtopics.
- Persist additional metrics (e.g., chunk depth, agent labels, KG token counts) in the store so the ranker has access without recomputation.
- Expose tuning knobs in configs so operators can adjust weights per route or per agent.

**Integration Notes:** These extensions keep the score transparent while acknowledging richer metadata from knowledge-graph chunking. They also dovetail with the router concept: each sub-engine can ship its preferred weight profile.

---

## 5. Agent & UI Hooks

**Goal:** Make Korel outputs turnkey for agentic RAG pipelines and enterprise dashboards.

**Proposal:**
- Define a stable JSON schema for cards (including chunk hierarchy, score breakdown, synonym explanations) and document it for MCP/tool integrations.
- Provide reference connectors (e.g., LangChain retriever, MCP tool) that call Korel’s API/router and return grounded evidence blocks ready for LLM synthesis.
- Offer a minimal web UI (or CLI) showcasing agent filters, card views, and “why this card” explanations, reinforcing the deterministic workflow.

**Integration Notes:** Even if full-fledged UI/agents live elsewhere, first-party tools demonstrate how Korel acts as the grounding engine at the center of an AI product, aligning with xLLM’s emphasis on explainable enterprise UX.

---

## Implementation Considerations

- **Data Model Changes:** Introduce chunk tables, synonym dictionaries, and c-token maps in `pkg/korel/store/sqlite` (or compatible storage backends). Ensure migration paths for existing deployments.
- **Performance:** The nested-hash approach described by Granville can be mirrored by caching indexes in memory per domain. Profiling will be needed for large corpora; consider memory-mapped files or compressed posting lists.
- **Testing:** Each feature should ship with synthetic corpora in `testdata/` to validate explainability (e.g., verifying that a synonym expansion is reported in score breakdowns).
- **Documentation:** Update `docs/BOOTSTRAP.md`, `README.md`, and Quick Start guides to reflect new flags (chunking options, synonym export, router config) once implemented.

---

By sequencing these enhancements, Korel evolves from a statistical/symbolic retrieval library into a full-fledged xLLM-style correlation engine suitable for AI agents, enterprise RAG stacks, and multi-domain deployments—while preserving the core principles of determinism, transparency, and minimal hardware footprint.
