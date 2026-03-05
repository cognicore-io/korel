package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/cognicore/korel/internal/bootstrap"
	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel/analytics"
	reviewllm "github.com/cognicore/korel/pkg/korel/autotune/review/llm"
	"github.com/cognicore/korel/pkg/korel/autotune/stopwords"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

type analyzeReport struct {
	TotalDocs          int64                       `json:"total_docs"`
	StopwordCandidates []analyzeStopwordCandidate  `json:"stopword_candidates"`
	HighDFTokens       []analytics.HighDFEntry     `json:"high_df_tokens"`
}

type analyzeStopwordCandidate struct {
	Token string  `json:"token"`
	Score float64 `json:"score"`
}

func runAnalyze() {
	var (
		input     = flag.String("input", "", "Path to JSONL file (required)")
		stopCfg   = flag.String("stoplist", "", "Stoplist file (required)")
		dictCfg   = flag.String("dict", "", "Dictionary file (required)")
		taxCfg    = flag.String("taxonomy", "", "Taxonomy file (required)")
		llmBase   = flag.String("llm-base", "", "Optional: OpenAI-compatible reviewer base URL")
		llmAPIKey = flag.String("llm-api-key", "", "Optional: API key for reviewer endpoint")
		llmLimit  = flag.Int("llm-limit", 10, "Maximum candidates to send to reviewer")
	)
	flag.Parse()

	if *input == "" {
		log.Fatal("--input required")
	}
	if *stopCfg == "" {
		log.Fatal("--stoplist required")
	}
	if *dictCfg == "" {
		log.Fatal("--dict required")
	}
	if *taxCfg == "" {
		log.Fatal("--taxonomy required")
	}

	ctx := context.Background()

	pl, err := bootstrap.LoadPipeline(bootstrap.Options{
		StoplistPath: *stopCfg,
		DictPath:     *dictCfg,
		TaxonomyPath: *taxCfg,
	})
	if err != nil {
		log.Fatalf("load configs: %v", err)
	}

	items, err := rss.LoadFromJSONL(*input)
	if err != nil {
		log.Fatalf("load docs: %v", err)
	}

	analyzer := analytics.NewAnalyzer()
	for _, item := range items {
		processed := pl.Pipeline.Process(item.Body)
		analyzer.Process(processed.Tokens, processed.Categories)
	}

	stats := analyzer.Snapshot()
	report := analyzeReport{
		TotalDocs: stats.TotalDocs,
	}
	report.HighDFTokens = analytics.TopHighDF(stats, 20)

	provider := analytics.NewStopwordStatsProvider(stats)
	tuner := stopwords.AutoTuner{
		Provider: provider,
		Manager:  stoplist.NewManager([]string{}),
	}

	candidates, err := tuner.Run(ctx)
	if err == nil && len(candidates) > 0 {
		reviewer := &reviewllm.Client{Endpoint: *llmBase, APIKey: *llmAPIKey}
		candidates = reviewer.ReviewStopwords(ctx, candidates, *llmLimit)
		for _, cand := range candidates {
			report.StopwordCandidates = append(report.StopwordCandidates, analyzeStopwordCandidate{
				Token: cand.Token,
				Score: cand.Score,
			})
		}
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("marshal report: %v", err)
	}
	fmt.Println(string(out))
}
