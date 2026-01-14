package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cognicore/korel/internal/llm"
	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

type agentConfig struct {
	DBPath   string `yaml:"db_path"`
	Stoplist string `yaml:"stoplist"`
	Dict     string `yaml:"dict"`
	Taxonomy string `yaml:"taxonomy"`
	Rules    string `yaml:"rules"`
	TopK     int    `yaml:"top_k"`

	LLM struct {
		BaseURL string `yaml:"base_url"`
		Model   string `yaml:"model"`
		APIKey  string `yaml:"api_key"`
	} `yaml:"llm"`
}

func main() {
	configPath := flag.String("config", "", "Path to agent config YAML")
	query := flag.String("query", "", "Question to ask (required)")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config required")
	}
	if *query == "" {
		log.Fatal("--query required")
	}

	conf, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	engine, cleanup, err := buildEngine(ctx, conf)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	llmClient := &llm.Client{
		BaseURL: conf.LLM.BaseURL,
		Model:   conf.LLM.Model,
		APIKey:  conf.LLM.APIKey,
	}

	topK := conf.TopK
	if topK <= 0 {
		topK = 3
	}

	result, err := engine.Search(ctx, korel.SearchRequest{
		Query: *query,
		TopK:  topK,
		Now:   time.Now(),
	})
	if err != nil {
		log.Fatalf("search: %v", err)
	}
	if len(result.Cards) == 0 {
		fmt.Println("No facts found.")
		return
	}

	printCards(result.Cards)

	if conf.LLM.BaseURL == "" || conf.LLM.Model == "" {
		return
	}

	summary, err := llmClient.Summarize(ctx, *query, result.Cards)
	if err != nil {
		log.Fatalf("llm summarize: %v", err)
	}
	fmt.Println("LLM Summary:")
	fmt.Println(summary)
}

func loadConfig(path string) (agentConfig, error) {
	var cfg agentConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func buildEngine(ctx context.Context, conf agentConfig) (*korel.Korel, func(), error) {
	if conf.DBPath == "" || conf.Stoplist == "" || conf.Dict == "" || conf.Taxonomy == "" {
		return nil, nil, errors.New("db_path, stoplist, dict, taxonomy required in config")
	}

	loader := config.Loader{
		StoplistPath: conf.Stoplist,
		DictPath:     conf.Dict,
		TaxonomyPath: conf.Taxonomy,
		RulesPath:    conf.Rules,
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
			return nil, nil, err
		}
		if err := inf.LoadRules(string(data)); err != nil {
			return nil, nil, err
		}
	}

	store, err := sqlite.OpenSQLite(ctx, conf.DBPath)
	if err != nil {
		return nil, nil, err
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

func printCards(cards []korel.Card) {
	out := struct {
		Cards []korel.Card `json:"cards"`
	}{Cards: cards}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
