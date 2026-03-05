package korel

import (
	"context"
	"fmt"
	"strings"

	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store"
)

// WarmInference loads persisted edges into the inference engine.
// If PMI edges already exist in the store, it just loads rule-worthy edges (fast).
// If the store has no PMI edges, it calls BuildGraph first (slow, one-time).
func (k *Korel) WarmInference(ctx context.Context) error {
	edges, err := k.store.AllEdges(ctx)
	if err != nil {
		return fmt.Errorf("load edges: %w", err)
	}
	// Check if PMI edges exist — those are the bulk of the graph.
	hasPMI := false
	for _, e := range edges {
		if e.Relation == "related_to" {
			hasPMI = true
			break
		}
	}
	if !hasPMI {
		// First time or after purge — build the full graph
		return k.BuildGraph(ctx)
	}
	k.loadRuleEdges(edges)
	return nil
}

// loadRuleEdges feeds non-PMI edges into the inference engine.
// Bulk PMI-generated related_to edges (source="pmi") stay in SQLite only —
// the search path already uses TopNeighbors for PMI lookup, and loading 30K+
// facts into Prolog makes assertz and backtracking prohibitively slow.
// AutoTune-generated related_to edges (source="autotune") ARE loaded because
// they are high-confidence curated rules that should survive restarts.
func (k *Korel) loadRuleEdges(edges []store.Edge) {
	for _, e := range edges {
		if e.Relation == "related_to" && e.Source == "pmi" {
			continue
		}
		k.inf.AddFact(e.Relation, e.Subject, e.Object)
	}
}

// pipelineTaxonomyKeywords returns category→keywords from all pipeline taxonomy types.
func (k *Korel) pipelineTaxonomyKeywords() map[string][]string {
	m := make(map[string][]string)
	for cat, kws := range k.pipeline.TaxonomySectors() {
		m[cat] = append(m[cat], kws...)
	}
	for cat, kws := range k.pipeline.TaxonomyEvents() {
		m[cat] = append(m[cat], kws...)
	}
	for cat, kws := range k.pipeline.TaxonomyRegions() {
		m[cat] = append(m[cat], kws...)
	}
	return m
}

// BuildGraph populates the persistent edges table from all knowledge sources
// (PMI co-occurrence, taxonomy, dictionary) and loads them into the inference
// engine. Idempotent: clears auto-generated edges before repopulating.
func (k *Korel) BuildGraph(ctx context.Context) error {
	// 1. Clear old auto-generated edges
	for _, src := range []string{"pmi", "taxonomy", "dict"} {
		if err := k.store.DeleteEdgesBySource(ctx, src); err != nil {
			return fmt.Errorf("clear %s edges: %w", src, err)
		}
	}

	// 2. PMI edges: related_to(a, b) where NPMI >= MinPMI, both tokens
	// appear in [MinDF, maxDF] documents. Stopwords are excluded.
	// Tokens appearing in too many docs are too generic for useful edges.
	gcfg := k.graphCfg
	stopSet := make(map[string]struct{})
	if sl := k.store.Stoplist(); sl != nil {
		for _, w := range sl.AllStops() {
			stopSet[w] = struct{}{}
		}
	}
	tokens, err := k.store.AllTokens(ctx)
	if err != nil {
		return fmt.Errorf("all tokens: %w", err)
	}

	// Compute maxDF from the most frequent token (proxy for corpus size)
	// and the configured MaxDFPercent threshold.
	var peakDF int64
	for _, tok := range tokens {
		df, err := k.store.GetTokenDF(ctx, tok)
		if err != nil {
			continue
		}
		if df > peakDF {
			peakDF = df
		}
	}
	maxDF := int64(float64(peakDF) * gcfg.MaxDFPercent / 100.0)
	if maxDF < 20 {
		maxDF = 20
	}

	for _, tok := range tokens {
		if _, isStop := stopSet[tok]; isStop {
			continue
		}
		if len(tok) < gcfg.MinTokenLen {
			continue
		}
		df, err := k.store.GetTokenDF(ctx, tok)
		if err != nil || df < gcfg.MinDF || df > maxDF {
			continue
		}
		neighbors, err := k.store.TopNeighbors(ctx, tok, 10)
		if err != nil {
			continue
		}
		for _, nb := range neighbors {
			if nb.PMI < gcfg.MinPMI {
				continue
			}
			if _, isStop := stopSet[nb.Token]; isStop {
				continue
			}
			if len(nb.Token) < gcfg.MinTokenLen {
				continue
			}
			nbDF, err := k.store.GetTokenDF(ctx, nb.Token)
			if err != nil || nbDF < gcfg.MinDF || nbDF > maxDF {
				continue
			}
			if err := k.store.UpsertEdge(ctx, store.Edge{
				Subject: tok, Relation: "related_to", Object: nb.Token,
				Weight: nb.PMI, Source: "pmi",
			}); err != nil {
				return fmt.Errorf("upsert PMI edge %s→%s: %w", tok, nb.Token, err)
			}
		}
	}

	// 3. Taxonomy edges: category(keyword, category_name)
	// Source from the pipeline (always has taxonomy data), not the store
	// (taxonomy tables may be unpopulated).
	for cat, kws := range k.pipelineTaxonomyKeywords() {
		for _, kw := range kws {
			if err := k.store.UpsertEdge(ctx, store.Edge{
				Subject: kw, Relation: "category", Object: cat,
				Weight: 1.0, Source: "taxonomy",
			}); err != nil {
				return fmt.Errorf("upsert taxonomy edge %s→%s: %w", kw, cat, err)
			}
		}
	}

	// 4. Dictionary edges: synonym(variant, canonical) + is_a(canonical, category)
	// Source from the pipeline dict entries directly.
	for _, e := range k.pipeline.DictEntries() {
		canonical := strings.ToLower(e.Canonical)
		for _, v := range e.Variants {
			variant := strings.ToLower(v)
			if variant != canonical {
				if err := k.store.UpsertEdge(ctx, store.Edge{
					Subject: variant, Relation: "synonym", Object: canonical,
					Weight: 1.0, Source: "dict",
				}); err != nil {
					return fmt.Errorf("upsert synonym edge %s→%s: %w", variant, canonical, err)
				}
			}
		}
		if e.Category != "" {
			if err := k.store.UpsertEdge(ctx, store.Edge{
				Subject: canonical, Relation: "is_a", Object: e.Category,
				Weight: 1.0, Source: "dict",
			}); err != nil {
				return fmt.Errorf("upsert is_a edge %s→%s: %w", canonical, e.Category, err)
			}
		}
	}

	// 5. Load rule-worthy edges into inference engine.
	edges, err := k.store.AllEdges(ctx)
	if err != nil {
		return fmt.Errorf("load edges: %w", err)
	}
	k.loadRuleEdges(edges)

	return nil
}

// expandViaPMI uses corpus co-occurrence statistics to find tokens that are
// statistically related to the query tokens. Only known dictionary concepts
// pass the filter — random co-occurring words are discarded.
func (k *Korel) expandViaPMI(ctx context.Context, queryTokens []string) []string {
	var result []string
	for _, tok := range queryTokens {
		neighbors, err := k.store.TopNeighbors(ctx, tok, 10)
		if err != nil {
			continue
		}
		for _, nb := range neighbors {
			if nb.PMI > 0.1 && k.isConcept(nb.Token) {
				result = append(result, nb.Token)
			}
		}
	}
	return result
}

// RebuildPipeline constructs a new ingest pipeline from the store's current
// stoplist, dictionary, and taxonomy. Call after AutoTune to pick up newly
// discovered stopwords and rules.
func (k *Korel) RebuildPipeline() {
	var stopwords []string
	if sl := k.store.Stoplist(); sl != nil {
		stopwords = sl.AllStops()
	}

	var dictEntries []ingest.DictEntry
	if dv := k.store.Dict(); dv != nil {
		for _, e := range dv.AllEntries() {
			dictEntries = append(dictEntries, ingest.DictEntry{
				Canonical: e.Canonical,
				Category:  e.Category,
				Variants:  []string{e.Phrase},
			})
		}
	}

	tokenizer := ingest.NewTokenizer(stopwords)
	parser := ingest.NewMultiTokenParser(dictEntries)
	taxonomy := ingest.NewTaxonomy()

	if tv := k.store.Taxonomy(); tv != nil {
		for name, keywords := range tv.AllSectors() {
			taxonomy.AddSector(name, keywords)
		}
		for name, keywords := range tv.AllEvents() {
			taxonomy.AddEvent(name, keywords)
		}
		for name, keywords := range tv.AllRegions() {
			taxonomy.AddRegion(name, keywords)
		}
		for entType, names := range tv.AllEntities() {
			for name, keywords := range names {
				taxonomy.AddEntity(entType, name, keywords)
			}
		}
	}

	k.pipeline = ingest.NewPipeline(tokenizer, parser, taxonomy)
}
