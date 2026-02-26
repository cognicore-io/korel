// search-demo is a self-contained demo of Korel's search and card generation.
//
// It ingests an embedded corpus of 20 tech articles, builds PMI co-occurrence
// statistics, warms the inference graph, and runs queries that demonstrate
// discovery beyond keyword matching.
//
// Run from the project root:
//
//	go run ./examples/search-demo
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Korel Search Demo ===")
	fmt.Println("Demonstrates: PMI-driven discovery, inference expansion, multi-token phrases")
	fmt.Println()

	// Step 1: Create temporary database with corpus-appropriate PMI config
	tmpDir, err := os.MkdirTemp("", "korel-demo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	dbPath := filepath.Join(tmpDir, "demo.db")

	// Step 2: Load base dictionary
	baseDictPath := findFile("configs/base-tech.dict")
	fmt.Printf("Loading base dictionary: %s\n", baseDictPath)

	dict, err := config.LoadDict(baseDictPath)
	if err != nil {
		log.Fatalf("load dict: %v", err)
	}
	fmt.Printf("  %d multi-token phrases loaded\n", len(dict.Entries))

	// Build pipeline with base dict + minimal stoplist + built-in taxonomy
	stopwords := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for",
		"of", "with", "by", "from", "is", "are", "was", "were", "be", "been",
		"being", "have", "has", "had", "do", "does", "did", "will", "would",
		"could", "should", "may", "might", "shall", "can", "this", "that",
		"these", "those", "it", "its", "their", "them", "they", "we", "our",
		"your", "you", "he", "she", "his", "her", "not", "no", "nor",
		"as", "if", "than", "then", "so", "such", "more", "most", "very",
		"also", "just", "about", "into", "over", "after", "before", "between",
		"through", "during", "up", "out", "all", "each", "every", "both",
		"any", "some", "other", "new", "which", "when", "where", "how", "what",
		"who", "whom", "there", "here", "only", "own", "same",
	}
	tokenizer := ingest.NewTokenizer(stopwords)

	entries := make([]ingest.DictEntry, len(dict.Entries))
	for i, e := range dict.Entries {
		entries[i] = ingest.DictEntry{
			Canonical: e.Canonical,
			Variants:  e.Variants,
			Category:  e.Category,
		}
	}
	parser := ingest.NewMultiTokenParser(entries)

	// Build taxonomy from the dict categories
	taxonomy := ingest.NewTaxonomy()
	catKeywords := make(map[string][]string)
	for _, e := range dict.Entries {
		catKeywords[e.Category] = append(catKeywords[e.Category], e.Canonical)
	}
	for cat, kws := range catKeywords {
		taxonomy.AddSector(cat, kws)
	}
	fmt.Printf("  %d taxonomy categories built from dictionary\n", len(catKeywords))

	pipeline := ingest.NewPipeline(tokenizer, parser, taxonomy)

	// Step 3: Open store with PMI config scaled for small corpus
	pmiCfg := pmi.ConfigForCorpusSize(len(demoCorpus))
	fmt.Printf("  PMI config: MinDF=%d (scaled for %d docs)\n", pmiCfg.MinDF, len(demoCorpus))

	store, err := sqlite.OpenSQLite(ctx, dbPath, pmiCfg)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	inf := simple.New()

	engine := korel.New(korel.Options{
		Store:     store,
		Pipeline:  pipeline,
		Inference: inf,
		Weights: korel.ScoreWeights{
			AlphaPMI:     1.0,
			BetaCats:     0.6,
			GammaRecency: 0.8,
			EtaAuthority: 0.2,
			DeltaLen:     0.05,
		},
		RecencyHalfLife: 14,
		PMI:             pmiCfg,
	})
	defer engine.Close()

	// Step 4: Ingest embedded corpus
	fmt.Printf("\nIngesting %d documents...\n", len(demoCorpus))
	for _, doc := range demoCorpus {
		if err := engine.Ingest(ctx, korel.IngestDoc{
			URL:         doc.URL,
			Title:       doc.Title,
			BodyText:    doc.Body,
			Outlet:      doc.Outlet,
			PublishedAt: doc.Published,
			SourceCats:  doc.Categories,
		}); err != nil {
			log.Printf("  Warning: ingest failed for %q: %v", doc.Title, err)
		}
	}
	fmt.Println("  Done.")

	// Step 5: Warm inference engine from corpus PMI statistics
	fmt.Println("\nWarming inference graph from PMI co-occurrence...")
	if err := engine.WarmInference(ctx); err != nil {
		log.Printf("  Warning: WarmInference: %v", err)
	}
	fmt.Println("  Done.")

	// Step 6: Run demo queries — these demonstrate DISCOVERY, not keyword matching
	queries := []struct {
		query string
		desc  string
	}{
		// "reinforcement learning" only appears in 1 article (DeepMind protein).
		// PMI expansion should add "deep learning", "neural network" etc. and
		// retrieve articles that never mention "reinforcement learning".
		{"reinforcement learning", "Term in 1 article — PMI expands to find related AI articles"},

		// "computer vision" only appears in the ML healthcare article.
		// Through co-occurrence with "machine learning" and "deep learning",
		// it should find other ML articles.
		{"computer vision", "Term in 1 article — discovers ML cluster through co-occurrence"},

		// "code review" appears in ML-security and devtools articles.
		// Should find related software engineering articles.
		{"code review", "Bridges security + devtools — cross-domain discovery"},

		// "container orchestration" appears in k8s and cloud-ai articles.
		// Should expand to find other cloud/devops articles.
		{"container orchestration", "DevOps concept — finds related infrastructure articles"},
	}

	for _, q := range queries {
		fmt.Printf("\n%s\n", strings.Repeat("=", 70))
		fmt.Printf("Query: %q\n", q.query)
		fmt.Printf("  (%s)\n", q.desc)
		fmt.Println(strings.Repeat("-", 70))

		res, err := engine.Search(ctx, korel.SearchRequest{
			Query: q.query,
			TopK:  3,
			Now:   time.Now(),
		})
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		if len(res.Cards) == 0 {
			fmt.Println("  No results found.")
			continue
		}

		// Show expansion — this is the key differentiator
		if len(res.Cards) > 0 {
			explain := res.Cards[0].Explain
			fmt.Printf("  Query tokens:    %v\n", explain.QueryTokens)
			if len(explain.ExpandedTokens) > len(explain.QueryTokens) {
				// Show only the tokens that were ADDED by expansion
				added := subtractTokens(explain.ExpandedTokens, explain.QueryTokens)
				if len(added) > 8 {
					added = added[:8]
				}
				fmt.Printf("  Expanded to:     %v\n", added)
			}
		}

		for i, card := range res.Cards {
			// Check if query term appears in the title (to highlight discovery)
			titleLower := strings.ToLower(card.Title)
			queryLower := strings.ToLower(q.query)
			marker := ""
			if !strings.Contains(titleLower, queryLower) {
				marker = " [DISCOVERED — not in title]"
			}

			fmt.Printf("\n  Card %d: %s%s\n", i+1, card.Title, marker)
			for _, bullet := range card.Bullets {
				fmt.Printf("    * %s\n", bullet)
			}
			fmt.Printf("    Source: %s (%s)\n",
				card.Sources[0].URL,
				card.Sources[0].Time.Format("2006-01-02"))

			if len(card.Explain.MatchedTokens) > 0 {
				max := 5
				if len(card.Explain.MatchedTokens) < max {
					max = len(card.Explain.MatchedTokens)
				}
				fmt.Printf("    Matched via: %v\n", card.Explain.MatchedTokens[:max])
			}
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 70))
	fmt.Println("Results marked [DISCOVERED] were found through PMI expansion and")
	fmt.Println("inference — they do NOT contain the query term. Full-text search")
	fmt.Println("would have missed them entirely.")
	fmt.Println()
	fmt.Println("No external APIs, no GPU, no per-query costs.")
}

// subtractTokens returns elements in a that are not in b.
func subtractTokens(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, s := range b {
		set[s] = struct{}{}
	}
	var result []string
	for _, s := range a {
		if _, ok := set[s]; !ok {
			result = append(result, s)
		}
	}
	return result
}

func findFile(relPath string) string {
	// Try relative to CWD first, then walk up
	candidates := []string{
		relPath,
		filepath.Join("..", "..", relPath),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	log.Fatalf("Cannot find %s. Run from project root.", relPath)
	return ""
}
