package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

func main() {
	var (
		dbPath       = flag.String("db", "", "Database path (required)")
		stoplistPath = flag.String("stoplist", "", "Stoplist file (required)")
		dictPath     = flag.String("dict", "", "Dictionary file (required)")
		taxonomyPath = flag.String("taxonomy", "", "Taxonomy file (required)")
		rulesPath    = flag.String("rules", "", "Rules file (optional)")
		query        = flag.String("query", "", "One-shot query (non-interactive mode)")
		topK         = flag.Int("topk", 3, "Number of results to return")
	)
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("--db required")
	}
	if *stoplistPath == "" {
		log.Fatal("--stoplist required")
	}
	if *dictPath == "" {
		log.Fatal("--dict required")
	}
	if *taxonomyPath == "" {
		log.Fatal("--taxonomy required")
	}

	ctx := context.Background()

	engine, cleanup, err := buildEngine(ctx, *dbPath, *stoplistPath, *dictPath, *taxonomyPath, *rulesPath)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	// One-shot query mode
	if *query != "" {
		if err := executeQuery(ctx, engine, *query, *topK); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Interactive mode
	fmt.Println("===========================================")
	fmt.Println("  Korel Chat CLI")
	fmt.Println("  Knowledge-first Q&A")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Println("Type your question (Ctrl+D to exit):")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			continue
		}

		if err := executeQuery(ctx, engine, query, *topK); err != nil {
			fmt.Println("Error:", err)
		}
	}

	fmt.Println("\nGoodbye!")
}

func executeQuery(ctx context.Context, engine *korel.Korel, query string, topK int) error {
	res, err := engine.Search(ctx, korel.SearchRequest{
		Query: query,
		Cats:  nil,
		TopK:  topK,
		Now:   time.Now(),
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(res.Cards) == 0 {
		fmt.Println("No results found.")
		fmt.Println()
		return nil
	}

	for i, card := range res.Cards {
		fmt.Printf("\n--- Card %d: %s ---\n", i+1, card.Title)
		for _, bullet := range card.Bullets {
			fmt.Println("  •", bullet)
		}

		fmt.Println("\nSources:")
		for _, src := range card.Sources {
			fmt.Printf("  - %s (%s)\n", src.URL, src.Time.Format("2006-01-02"))
		}

		fmt.Println("\nScore Breakdown:")
		for k, v := range card.ScoreBreakdown {
			fmt.Printf("  %s: %.2f\n", k, v)
		}

		fmt.Println("\nExplain:")
		fmt.Printf("  Query tokens: %v\n", card.Explain.QueryTokens)
		fmt.Printf("  Expanded tokens: %v\n", card.Explain.ExpandedTokens)
		if len(card.Explain.MatchedTokens) > 0 {
			max := min(5, len(card.Explain.MatchedTokens))
			fmt.Printf("  Matched tokens: %v\n", card.Explain.MatchedTokens[:max])
		}
		if len(card.Explain.TopPairs) > 0 {
			fmt.Println("  Top pairs:")
			for _, p := range card.Explain.TopPairs[:min(3, len(card.Explain.TopPairs))] {
				fmt.Printf("    %v ↔ %v (PMI: %.2f)\n", p[0], p[1], p[2])
			}
		}
		fmt.Println()
	}

	return nil
}

func buildEngine(ctx context.Context, dbPath, stoplistPath, dictPath, taxonomyPath, rulesPath string) (*korel.Korel, func(), error) {
	loader := config.Loader{
		StoplistPath: stoplistPath,
		DictPath:     dictPath,
		TaxonomyPath: taxonomyPath,
		RulesPath:    rulesPath,
	}

	components, err := loader.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	pipeline := ingest.NewPipeline(components.Tokenizer, components.Parser, components.Taxonomy)

	inf := simple.New()
	if components.Rules != "" {
		data, err := os.ReadFile(components.Rules)
		if err != nil {
			return nil, nil, fmt.Errorf("read rules: %w", err)
		}
		if err := inf.LoadRules(string(data)); err != nil {
			return nil, nil, fmt.Errorf("load rules: %w", err)
		}
	}

	store, err := sqlite.OpenSQLite(ctx, dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}

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
			ThetaInfer:   0.3,
		},
		RecencyHalfLife: 14,
	})

	cleanup := func() {
		engine.Close()
	}

	return engine, cleanup, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
