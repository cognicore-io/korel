// Example: Taxonomy Drift Detection
//
// Demonstrates how Korel detects when a taxonomy no longer covers the corpus:
//
//   1. LOW-COVERAGE: Keywords in the taxonomy that rarely appear in the corpus.
//      Example: taxonomy says "animals" includes "dinosaur" but no docs mention it.
//
//   2. ORPHAN TOKENS: Frequent tokens NOT in any taxonomy category — new topics
//      the taxonomy hasn't caught up with.
//      Example: "robot" appears in 33% of docs but isn't categorized anywhere.
//
// Run:
//
//	go run ./examples/taxonomy-drift
package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/memstore"
)

// baseStopwords are common English words that aren't domain-relevant.
// In production these come from testdata/hn/stoplist.yaml or AutoTune discovery.
var baseStopwords = []string{
	"the", "a", "an", "is", "was", "are", "were", "be", "been", "being",
	"have", "has", "had", "do", "does", "did", "will", "would", "could",
	"should", "may", "might", "shall", "can", "need", "dare", "ought",
	"to", "of", "in", "for", "on", "with", "at", "by", "from", "as",
	"into", "through", "during", "before", "after", "above", "below",
	"between", "out", "off", "over", "under", "again", "further", "then",
	"once", "and", "but", "or", "nor", "not", "so", "yet", "both",
	"either", "neither", "each", "every", "all", "any", "few", "more",
	"most", "other", "some", "such", "no", "only", "own", "same",
	"than", "too", "very", "just", "because", "if", "when", "while",
	"that", "which", "who", "whom", "this", "these", "those",
	"it", "its", "he", "she", "his", "her", "they", "them", "their",
	"we", "our", "you", "your", "i", "me", "my", "up", "about",
}

func main() {
	ctx := context.Background()

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║  Korel: Taxonomy Drift Detection Demo           ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	// ── Step 1: Define a taxonomy ──────────────────────────────────────
	//
	// This taxonomy knows about "animals" and "food" but nothing about
	// robots or technology. Two keywords are stale — "dinosaur" and
	// "sushi" won't appear in the corpus.

	ms := memstore.New()
	ms.SetTaxonomy(
		map[string][]string{
			"animals": {"cat", "dog", "bird", "fish", "dinosaur"}, // "dinosaur" is stale
			"food":    {"cake", "cookie", "bread", "sushi"},       // "sushi" is stale
		},
		nil, nil, nil,
	)

	fmt.Println("Taxonomy configured:")
	fmt.Println("  animals: [cat, dog, bird, fish, dinosaur]   ← 'dinosaur' never appears")
	fmt.Println("  food:    [cake, cookie, bread, sushi]       ← 'sushi' never appears")
	fmt.Println()

	// Build pipeline with the same taxonomy for category assignment.
	tax := ingest.NewTaxonomy()
	tax.AddSector("animals", []string{"cat", "dog", "bird", "fish", "dinosaur"})
	tax.AddSector("food", []string{"cake", "cookie", "bread", "sushi"})

	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer(baseStopwords),
		ingest.NewMultiTokenParser(nil),
		tax,
	)

	engine := korel.New(korel.Options{
		Store:           ms,
		Pipeline:        pipeline,
		Inference:       simple.New(),
		Weights:         korel.ScoreWeights{AlphaPMI: 1},
		RecencyHalfLife: 14,
	})
	defer engine.Close()

	// ── Step 2: Build corpus ───────────────────────────────────────────
	//
	// - Animals and food keywords appear regularly.
	// - "dinosaur" and "sushi" are in taxonomy but never mentioned.
	// - "robot" and "laser" appear frequently but aren't in ANY category.
	// - Varying sentence patterns avoid making everything a stopword.

	fmt.Println("Building corpus (200 documents)...")
	corpus := buildCorpus()
	fmt.Printf("  50x cat/dog/bird/fish stories  (animals)\n")
	fmt.Printf("  50x cake/cookie/bread stories  (food)\n")
	fmt.Printf("  50x robot/laser stories        (NOT in taxonomy!)\n")
	fmt.Printf("  50x mixed stories              (variety)\n")
	fmt.Println()

	// ── Step 3: Run AutoTune ───────────────────────────────────────────

	fmt.Println("Running AutoTune (with base stopwords)...")
	result, err := engine.AutoTune(ctx, corpus, &korel.AutoTuneOptions{
		BaseStopwords: baseStopwords,
		MaxIterations: 2,
		Thresholds:    korel.AutoTuneDefaults(),
	})
	if err != nil {
		fmt.Printf("AutoTune error: %v\n", err)
		return
	}

	fmt.Printf("  Stopwords discovered: %d (on top of %d base)\n",
		len(result.StopwordCandidates), len(baseStopwords))
	fmt.Printf("  Rules discovered:     %d\n", len(result.RuleSuggestions))
	fmt.Printf("  Taxonomy suggestions: %d\n", len(result.TaxonomySuggestions))
	fmt.Println()

	// ── Step 4: Display drift report ───────────────────────────────────

	if len(result.TaxonomySuggestions) == 0 {
		fmt.Println("No taxonomy drift detected.")
		return
	}

	// Separate by type: stale (0 docs) vs declining (some docs but low) vs orphan.
	var stale, declining, orphans []taxonomy_line
	totalDocs := int64(len(corpus))
	for _, s := range result.TaxonomySuggestions {
		switch s.Type {
		case "low_coverage":
			appearsIn := totalDocs - s.MissedDocs
			pct := float64(appearsIn) * 100 / float64(totalDocs)
			entry := taxonomy_line{
				keyword: s.Keyword, category: s.Category,
				confidence: s.Confidence, count: appearsIn, pct: pct,
			}
			if appearsIn == 0 {
				stale = append(stale, entry)
			} else {
				declining = append(declining, entry)
			}
		case "orphan":
			cat := s.Category
			if cat == "" {
				cat = "(new category?)"
			}
			pct := float64(s.MissedDocs) * 100 / float64(totalDocs)
			orphans = append(orphans, taxonomy_line{
				keyword: s.Keyword, category: cat,
				confidence: s.Confidence, count: s.MissedDocs, pct: pct,
			})
		}
	}

	sort.Slice(stale, func(i, j int) bool { return stale[i].keyword < stale[j].keyword })
	sort.Slice(declining, func(i, j int) bool { return declining[i].pct < declining[j].pct })
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].pct > orphans[j].pct })

	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println("  TAXONOMY DRIFT REPORT")
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println()

	if len(stale) > 0 {
		fmt.Println("  STALE — Keywords in taxonomy that NEVER appear in corpus:")
		fmt.Println("  (Remove these or add content that uses them)")
		fmt.Println()
		for _, l := range stale {
			fmt.Printf("    %-14s  category=%-10s  0 docs — keyword is dead\n",
				l.keyword, l.category)
		}
		fmt.Println()
	}

	if len(declining) > 0 {
		fmt.Println("  LOW COVERAGE — Keywords present but rarely used:")
		fmt.Println("  (These cover <40%% of corpus — may indicate niche topics)")
		fmt.Println()
		for _, l := range declining {
			fmt.Printf("    %-14s  category=%-10s  %d docs (%.0f%% coverage)\n",
				l.keyword, l.category, l.count, l.pct)
		}
		fmt.Println()
	}

	if len(orphans) > 0 {
		fmt.Println("  ORPHAN TOKENS — Frequent terms not in any category:")
		fmt.Println("  (New topics the taxonomy should cover)")
		fmt.Println()
		for _, l := range orphans {
			fmt.Printf("    %-14s  suggest=%-14s  %d docs (%.0f%% of corpus)\n",
				l.keyword, l.category, l.count, l.pct)
		}
		fmt.Println()
	}

	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  What to do with these suggestions:")
	fmt.Println("    low_coverage → Remove stale keywords or add relevant content")
	fmt.Println("    orphan       → Add to the suggested category, or create a new one")
	fmt.Println("    Suggestions are NOT auto-applied — they require human review")
	fmt.Println()
}

type taxonomy_line struct {
	keyword    string
	category   string
	confidence float64
	count      int64
	pct        float64
}

func buildCorpus() []string {
	corpus := make([]string, 0, 200)

	// 50 animal stories (cat, dog, bird, fish appear frequently).
	animals := []string{
		"cat chased mouse around garden fence",
		"dog fetched ball across park field",
		"bird sang melody from tall tree branch",
		"fish swam through coral reef ocean",
		"cat played yarn living room afternoon",
		"dog barked loudly stranger approached gate",
		"bird built nest spring tree leaves",
		"fish jumped water caught fly insect",
		"cat napped sunny window warm blanket",
		"dog dug hole backyard bone buried",
	}
	for i := 0; i < 50; i++ {
		corpus = append(corpus, animals[i%len(animals)])
	}

	// 50 food stories (cake, cookie, bread appear frequently).
	foods := []string{
		"cake baked oven chocolate frosting layers",
		"cookie dough mixed flour sugar butter",
		"bread dough risen overnight baked morning",
		"cake decorated flowers birthday party celebration",
		"cookie sheet placed oven timer twenty",
		"bread sliced fresh butter honey drizzled",
		"cake recipe grandmother passed down generations",
		"cookie crumbled pie crust cheesecake base",
		"bread loaf sourdough starter fermented days",
		"cake frosted cream cheese vanilla extract",
	}
	for i := 0; i < 50; i++ {
		corpus = append(corpus, foods[i%len(foods)])
	}

	// 50 robot/technology stories (NOT in taxonomy — should be orphans).
	tech := []string{
		"robot assembled circuit board factory floor",
		"laser beam cut metal precision workshop",
		"robot programmed navigate warehouse shelves automatically",
		"laser scanner measured distance target sensor",
		"robot arm welded steel frame construction",
		"laser printer produced high resolution output",
		"robot vacuum cleaned floor charging station",
		"laser pointer guided presentation audience screen",
		"robot companion helped elderly patient daily",
		"laser surgery performed eye correction procedure",
	}
	for i := 0; i < 50; i++ {
		corpus = append(corpus, tech[i%len(tech)])
	}

	// 50 mixed stories (variety — keeps stats realistic).
	mixed := []string{
		"garden flowers bloomed spring rain sunshine",
		"mountain trail hiked summit view panoramic",
		"river flowed valley bridge crossed town",
		"library books shelved readers browsing quiet",
		"concert music played stage audience cheered",
		"market vendors sold produce fresh morning",
		"school children learned classroom teacher taught",
		"hospital doctors treated patients care medical",
		"airport flights departed gate travelers waiting",
		"stadium crowd watched game team scored",
	}
	for i := 0; i < 50; i++ {
		corpus = append(corpus, mixed[i%len(mixed)])
	}

	return corpus
}
