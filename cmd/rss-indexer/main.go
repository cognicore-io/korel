package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

func main() {
	var (
		dbPath       = flag.String("db", "", "Database path (required)")
		dataPath     = flag.String("data", "", "Input JSONL file (required)")
		stoplistPath = flag.String("stoplist", "", "Stoplist file (required)")
		dictPath     = flag.String("dict", "", "Dictionary file (required)")
		taxonomyPath = flag.String("taxonomy", "", "Taxonomy file (required)")
		rulesPath    = flag.String("rules", "", "Rules file (optional)")
	)
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("--db required")
	}
	if *dataPath == "" {
		log.Fatal("--data required")
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

	// Load configuration components
	loader := config.Loader{
		StoplistPath: *stoplistPath,
		DictPath:     *dictPath,
		TaxonomyPath: *taxonomyPath,
		RulesPath:    *rulesPath,
	}

	components, err := loader.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Create ingestion pipeline
	pipeline := ingest.NewPipeline(components.Tokenizer, components.Parser, components.Taxonomy)

	// Initialize inference engine
	inf := simple.New()
	if components.Rules != "" {
		rulesData, err := os.ReadFile(components.Rules)
		if err != nil {
			log.Fatal("Failed to read rules:", err)
		}
		if err := inf.LoadRules(string(rulesData)); err != nil {
			log.Fatal("Failed to load rules:", err)
		}
	}

	// Open database
	store, err := sqlite.OpenSQLite(ctx, *dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer store.Close()

	// Create Korel instance
	k := korel.New(korel.Options{
		Store:    store,
		Pipeline: pipeline,
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
	defer k.Close()

	log.Println("Korel RSS Indexer started")

	// Load documents from JSONL
	items, err := rss.LoadFromJSONL(*dataPath)
	if err != nil {
		log.Fatal("Failed to load documents:", err)
	}

	log.Printf("Loaded %d documents from %s", len(items), *dataPath)

	// Ingest documents
	for i, item := range items {
		doc := korel.IngestDoc{
			URL:         item.URL,
			Title:       item.Title,
			Outlet:      item.Outlet,
			PublishedAt: item.PublishedAt,
			BodyText:    item.Body,
			SourceCats:  item.SourceCats,
		}

		if err := k.Ingest(ctx, doc); err != nil {
			log.Printf("Failed to ingest document %d (%s): %v", i, doc.Title, err)
			continue
		}

		if (i+1)%10 == 0 {
			log.Printf("Ingested %d/%d documents", i+1, len(items))
		}
	}

	log.Printf("✓ Indexing complete: %d documents processed", len(items))
}
