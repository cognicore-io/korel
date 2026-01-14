# Korel Autotune Modules

The `autotune` packages host **optional self-tuning jobs** that operate alongside the
core ingestion/search pipeline.  Each module consumes analytics snapshots, proposes
configuration updates (stopwords, taxonomy rules, symbolic facts, …), and can be
plugged into background workers or maintenance CLIs.

## Design Principles

1. **Modular services** – every tuner lives in its own package, exposes a tiny interface,
   and has zero direct dependencies on `cmd/` binaries or SQLite specifics.
2. **Analytics-first** – tuners depend on read-only stats providers (document frequency
   summaries, PMI heatmaps, drift scores).  This keeps them testable and storage agnostic.
3. **Human-in-the-loop optionality** – each tuner can run in “automatic” mode or route
   suggestions through an external reviewer (human or LLM) before applying changes.
4. **Idempotent outputs** – tuners only _suggest_ actions (e.g., `stoplist.Manager.Add`).
   The caller decides when/how to persist new configs (write YAML, commit to Git, etc.).

## Available Modules

### `autotune/stopwords`

Suggests new stopwords from aggregated statistics:

- Inputs: document frequency %, PMI maxima, category entropy.
- Optional validator: external LLM or human callback.
- Output: ranked `stoplist.Candidate` items ready for review or automatic addition.

Usage pattern:

```go
provider := analytics.NewStopwordStatsProvider(store)
manager  := stoplist.NewManager(existingTerms)
tuner    := stopwords.AutoTuner{
    Provider:   provider,
    Manager:    manager,
    Thresholds: stoplist.DefaultThresholds(),
    Reviewer:   myLLMReviewer, // optional
}
suggestions, _ := tuner.Run(ctx)
```

### `autotune/taxonomy`

Flags category drift events (e.g., sector docs lacking canonical keywords) and proposes
new keywords to add.  Suggestions include confidence scores so operators or reviewers
can prioritize them, and the module accepts the same reviewer pattern for approvals.

### Reviewer helpers

`autotune/review/llm` implements a reviewer that calls an external LLM endpoint.  It
formats prompts describing each candidate (stopword or taxonomy keyword) and expects
a JSON `{ "approve": true|false }` response.  This keeps the core tuners deterministic
while enabling optional human/LLM veto power.

### `autotune/rules`

Mines PMI/co-occurrence stats for strong token pairs and emits candidate symbolic facts
(e.g., `related_to(machine-learning, neural-network)`).  Approved suggestions can be
fed back into the inference engine’s rule base.

### `autotune/entities`

Bootstraps or updates entity dictionaries by looking for high-confidence mentions in
ingested documents (company names, products, etc.).  Outputs normalized names plus
keyword variants that can be appended to taxonomy/entity configs.

Each module will follow the same pattern: inject analytics providers + optional reviewers,
return structured suggestions, and let the caller persist or broadcast the decisions.

## External LLM / Human Review

Some environments may want LLM or expert review before mutating configs.  Tuners accept
an optional **Reviewer** interface; when provided, every suggestion is passed to the
reviewer for approval/annotation (e.g., “LLM says token ‘press-release’ is generic”).

The default behavior (no reviewer) is deterministic, driven purely by statistics, and
safe to run inside automated maintenance jobs.

## Testing

Every autotune package ships with focused unit tests that exercise the statistical
logic and reviewer flow via in-memory fakes.  No real database or LLM calls are
needed to validate the behavior.
