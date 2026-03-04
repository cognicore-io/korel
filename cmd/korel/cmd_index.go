package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/cognicore/korel/internal/bootstrap"
	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel"
)

func runIndex() {
	var (
		dbPath          = flag.String("db", "", "Database path (required)")
		dataPath        = flag.String("data", "", "Input JSONL file (required)")
		stoplistPath    = flag.String("stoplist", "", "Stoplist file (required)")
		dictPath        = flag.String("dict", "", "Dictionary file (required)")
		baseDictPath    = flag.String("base-dict", "", "Base dictionary to merge (e.g., configs/base-tech.dict)")
		taxonomyPath    = flag.String("taxonomy", "", "Taxonomy file (required)")
		rulesPath       = flag.String("rules", "", "Rules file (optional)")
		simpleInference = flag.Bool("simple-inference", false, "Use simple inference engine instead of Prolog")
		skipWarm        = flag.Bool("skip-warm", false, "Skip WarmInference after ingestion (faster, no edges)")
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

	log.Println("[BOOTSTRAP] Starting engine...")

	// Bootstrap engine (skip WarmInference — we call it after ingestion)
	res, err := bootstrap.Run(ctx, bootstrap.Options{
		DBPath:          *dbPath,
		StoplistPath:    *stoplistPath,
		DictPath:        *dictPath,
		BaseDictPath:    *baseDictPath,
		TaxonomyPath:    *taxonomyPath,
		RulesPath:       *rulesPath,
		SimpleInference: *simpleInference,
		SkipWarm:        true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	log.Println("[BOOTSTRAP] Engine ready")

	// Load documents from JSONL
	log.Printf("[LOAD] Reading %s...", *dataPath)
	items, err := rss.LoadFromJSONL(*dataPath)
	if err != nil {
		log.Fatal("Failed to load documents:", err)
	}
	log.Printf("[LOAD] %d documents loaded", len(items))

	// Ingest documents with detailed progress
	total := len(items)
	startTime := time.Now()
	errors := 0

	log.Printf("[INGEST] Starting ingestion of %d documents...", total)

	for i, item := range items {
		docStart := time.Now()

		doc := korel.IngestDoc{
			URL:         item.URL,
			Title:       item.Title,
			Outlet:      item.Outlet,
			PublishedAt: item.PublishedAt,
			BodyText:    item.Body,
			SourceCats:  item.SourceCats,
		}

		if err := res.Engine.Ingest(ctx, doc); err != nil {
			log.Printf("[INGEST] ERROR doc %d/%d (%s): %v", i+1, total, doc.URL, err)
			errors++
			continue
		}

		docDur := time.Since(docStart)
		elapsed := time.Since(startTime)
		docsPerSec := float64(i+1) / elapsed.Seconds()
		remaining := time.Duration(float64(total-i-1)/docsPerSec) * time.Second

		// Log every document
		log.Printf("[INGEST] %d/%d (%.0f%%) %s  [%.0fms] [%.1f docs/s] [ETA %s]",
			i+1, total,
			100*float64(i+1)/float64(total),
			doc.URL,
			float64(docDur.Milliseconds()),
			docsPerSec,
			formatDuration(remaining),
		)
	}

	elapsed := time.Since(startTime)
	log.Printf("[INGEST] Done: %d/%d documents ingested, %d errors, %s elapsed",
		total-errors, total, errors, formatDuration(elapsed))

	// Build knowledge graph
	if *skipWarm {
		log.Println("[WARM] Skipped (--skip-warm)")
	} else {
		log.Println("[WARM] Building knowledge graph (edges + inference)...")
		warmStart := time.Now()
		if err := res.Engine.WarmInference(ctx); err != nil {
			log.Printf("[WARM] ERROR: %v", err)
		} else {
			log.Printf("[WARM] Done in %s", formatDuration(time.Since(warmStart)))
		}
	}

	log.Printf("[DONE] Indexing complete: %d documents, %s total", total, formatDuration(time.Since(startTime)))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
