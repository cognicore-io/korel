package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sort"

	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel/analytics"
	reviewllm "github.com/cognicore/korel/pkg/korel/autotune/review/llm"
	"github.com/cognicore/korel/pkg/korel/autotune/stopwords"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

type report struct {
	TotalDocs          int64                   `json:"total_docs"`
	StopwordCandidates []stoplistCandidateJSON `json:"stopword_candidates"`
	HighDFTokens       []highDFEntry           `json:"high_df_tokens"`
}

type stoplistCandidateJSON struct {
	Token string  `json:"token"`
	Score float64 `json:"score"`
}

type highDFEntry struct {
	Token     string  `json:"token"`
	DFPercent float64 `json:"df_percent"`
	Entropy   float64 `json:"entropy"`
}

func main() {
	var (
		input       = flag.String("input", "", "Path to JSONL file (required)")
		stoplistCfg = flag.String("stoplist", "", "Stoplist file (required)")
		dictCfg     = flag.String("dict", "", "Dictionary file (required)")
		taxCfg      = flag.String("taxonomy", "", "Taxonomy file (required)")
		llmBase     = flag.String("llm-base", "", "Optional: OpenAI-compatible reviewer base URL")
		llmModel    = flag.String("llm-model", "", "Optional: LLM model name for reviewer")
		llmAPIKey   = flag.String("llm-api-key", "", "Optional: API key for reviewer endpoint")
		llmLimit    = flag.Int("llm-limit", 10, "Maximum candidates to send to reviewer")
	)
	flag.Parse()

	if *input == "" {
		log.Fatal("--input required")
	}
	if *stoplistCfg == "" {
		log.Fatal("--stoplist required")
	}
	if *dictCfg == "" {
		log.Fatal("--dict required")
	}
	if *taxCfg == "" {
		log.Fatal("--taxonomy required")
	}

	ctx := context.Background()

	loader := config.Loader{
		StoplistPath: *stoplistCfg,
		DictPath:     *dictCfg,
		TaxonomyPath: *taxCfg,
	}

	components, err := loader.Load()
	if err != nil {
		log.Fatalf("load configs: %v", err)
	}

	pipeline := ingest.NewPipeline(components.Tokenizer, components.Parser, components.Taxonomy)

	items, err := rss.LoadFromJSONL(*input)
	if err != nil {
		log.Fatalf("load docs: %v", err)
	}

	analyzer := analytics.NewAnalyzer()
	for _, item := range items {
		processed := pipeline.Process(item.Body)
		analyzer.Process(processed.Tokens, processed.Categories)
	}

	stats := analyzer.Snapshot()
	report := report{
		TotalDocs: stats.TotalDocs,
	}
	report.HighDFTokens = topHighDF(stats, 20)

	// reuse stopword autotuner to rank candidates
	provider := analytics.NewStopwordStatsProvider(stats)
	tuner := stopwords.AutoTuner{
		Provider: provider,
		Manager:  stoplist.NewManager([]string{}),
	}

	candidates, err := tuner.Run(ctx)
	if err == nil && len(candidates) > 0 {
		candidates = applyLLMReviewer(ctx, candidates, *llmBase, *llmModel, *llmAPIKey, *llmLimit)
		for _, cand := range candidates {
			report.StopwordCandidates = append(report.StopwordCandidates, stoplistCandidateJSON{
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

func topHighDF(stats analytics.Stats, limit int) []highDFEntry {
	stopStats := stats.StopwordStats()
	sort.Slice(stopStats, func(i, j int) bool {
		return stopStats[i].DFPercent > stopStats[j].DFPercent
	})
	if limit > 0 && len(stopStats) > limit {
		stopStats = stopStats[:limit]
	}
	out := make([]highDFEntry, 0, len(stopStats))
	for _, stat := range stopStats {
		out = append(out, highDFEntry{
			Token:     stat.Token,
			DFPercent: stat.DFPercent,
			Entropy:   stat.CatEntropy,
		})
	}
	return out
}

func applyLLMReviewer(ctx context.Context, candidates []stoplist.Candidate, baseURL, model, apiKey string, limit int) []stoplist.Candidate {
	if baseURL == "" || model == "" {
		return candidates
	}
	reviewer := &reviewllm.Client{
		Endpoint: baseURL,
		APIKey:   apiKey,
	}
	var result []stoplist.Candidate
	for i, cand := range candidates {
		if limit <= 0 || i < limit {
			ok, err := reviewer.Approve(ctx, cand)
			if err != nil {
				log.Printf("review error for %s: %v", cand.Token, err)
				continue
			}
			if ok {
				result = append(result, cand)
			}
		} else {
			result = append(result, cand)
		}
	}
	return result
}
