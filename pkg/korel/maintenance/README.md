# Korel Maintenance Jobs

This package provides background workers that keep the stored corpus aligned with
evolving configurations (stopwords, taxonomy keywords, symbolic rules).

## Workflow

1. **Ingestion** – runs with the current stoplist/dictionary/taxonomy.
2. **Autotuners** – stopwords, taxonomy, rules, and entities modules analyze corpus
   statistics and propose updates (optionally reviewed by humans or LLMs).
3. **Maintenance** – the jobs in this package reprocess only the affected documents:
   - Remove newly added stopwords from token arrays, update DF/PMI counts.
   - Add taxonomy keywords/entities when new canonical forms were approved.
   - Export newly approved rules to the inference engine (or Prolog files).
4. **Reload configs** – ingestion/search processes reload the updated files before
   the next batch.

By separating these phases, ingestion stays fast while the corpus remains clean and
deterministic.

## Components

- `Cleaner`: finds documents containing newly-added stopwords and replays them through
  the tokenizer/multi-token pipeline, removing those tokens and adjusting stats.
- `RuleExporter`: writes approved symbolic rules into Prolog-compatible text blocks.
- (Future) Taxonomy/entity maintenance jobs can reuse the same `Cleaner` scaffolding.

Each job is idempotent and chunked so it can resume after interruptions.

