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

	"gopkg.in/yaml.v3"

	"github.com/cognicore/korel/internal/bootstrap"
	"github.com/cognicore/korel/internal/llm"
	"github.com/cognicore/korel/pkg/korel"
)

// parseTime parses a date string as either RFC3339 or YYYY-MM-DD.
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// searchConfig holds YAML config for the search subcommand (optional).
type searchConfig struct {
	DBPath   string `yaml:"db_path"`
	Stoplist string `yaml:"stoplist"`
	Dict     string `yaml:"dict"`
	Taxonomy string `yaml:"taxonomy"`
	Rules    string `yaml:"rules"`
	TopK     int    `yaml:"top_k"`

	Scoring *scoringConfig `yaml:"scoring"`

	LLM struct {
		BaseURL string `yaml:"base_url"`
		Model   string `yaml:"model"`
		APIKey  string `yaml:"api_key"`
	} `yaml:"llm"`
}

// scoringConfig allows tuning scoring weights without recompiling.
type scoringConfig struct {
	PMI       *float64 `yaml:"pmi"`       // PMI co-occurrence weight (default 1.0)
	BM25      *float64 `yaml:"bm25"`      // BM25 term relevance weight (default 0.8)
	Title     *float64 `yaml:"title"`     // Title match boost (default 5.0)
	Cats      *float64 `yaml:"cats"`      // Category overlap weight (default 0.6)
	Recency   *float64 `yaml:"recency"`   // Recency decay weight (default 0.8)
	Authority *float64 `yaml:"authority"` // Link authority weight (default 0.2)
	Len       *float64 `yaml:"len"`       // Length penalty weight (default 0.05)

	// BM25 parameters
	BM25K1 *float64 `yaml:"bm25_k1"` // Term frequency saturation (default 1.2)
	BM25B  *float64 `yaml:"bm25_b"`  // Length normalization (default 0.35)
}

func runSearch() {
	var (
		configPath      = flag.String("config", "", "Path to YAML config (alternative to individual flags)")
		dbPath          = flag.String("db", "", "Database path (required unless --config)")
		stoplistPath    = flag.String("stoplist", "", "Stoplist file (required unless --config)")
		dictPath        = flag.String("dict", "", "Dictionary file (required unless --config)")
		baseDictPath    = flag.String("base-dict", "", "Base dictionary to merge")
		taxonomyPath    = flag.String("taxonomy", "", "Taxonomy file (required unless --config)")
		rulesPath       = flag.String("rules", "", "Rules file (optional)")
		query           = flag.String("query", "", "One-shot query (non-interactive mode)")
		topK            = flag.Int("topk", 3, "Number of results to return")
		simpleInference = flag.Bool("simple-inference", false, "Use simple inference engine instead of Prolog")

		// Feature flags
		searchMode  = flag.String("mode", "", "Search mode: fact, trend, compare, explore (default: auto)")
		sinceStr    = flag.String("since", "", "Time lower bound (RFC3339 or YYYY-MM-DD)")
		untilStr    = flag.String("until", "", "Time upper bound (RFC3339 or YYYY-MM-DD)")
		entity      = flag.String("entity", "", "Focus results on this entity")
		maxHops     = flag.Int("max-hops", 0, "Inference hop limit (0=default, -1=disable)")
		relationsStr = flag.String("relations", "", "Comma-separated allowed inference relations")
		rewrite     = flag.Bool("rewrite", false, "Enable query rewriting via dictionary")
		format      = flag.String("format", "", "Output format: briefing, memo, digest, watchlist")
	)
	flag.Parse()

	// Load YAML config if provided; CLI flags override config values
	var conf searchConfig
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
		if err := yaml.Unmarshal(data, &conf); err != nil {
			log.Fatalf("parse config: %v", err)
		}
	}
	if *dbPath != "" {
		conf.DBPath = *dbPath
	}
	if *stoplistPath != "" {
		conf.Stoplist = *stoplistPath
	}
	if *dictPath != "" {
		conf.Dict = *dictPath
	}
	if *taxonomyPath != "" {
		conf.Taxonomy = *taxonomyPath
	}
	if *rulesPath != "" {
		conf.Rules = *rulesPath
	}
	if *topK != 3 || conf.TopK <= 0 {
		conf.TopK = *topK
	}

	if conf.DBPath == "" || conf.Stoplist == "" || conf.Dict == "" || conf.Taxonomy == "" {
		log.Fatal("--db, --stoplist, --dict, --taxonomy required (via flags or --config)")
	}

	ctx := context.Background()

	// Apply scoring weights from config (if present)
	var weights korel.ScoreWeights
	if conf.Scoring != nil {
		weights = korel.DefaultScoreWeights()
		s := conf.Scoring
		if s.PMI != nil {
			weights.AlphaPMI = *s.PMI
		}
		if s.BM25 != nil {
			weights.ZetaBM25 = *s.BM25
		}
		if s.Title != nil {
			weights.IotaTitle = *s.Title
		}
		if s.Cats != nil {
			weights.BetaCats = *s.Cats
		}
		if s.Recency != nil {
			weights.GammaRecency = *s.Recency
		}
		if s.Authority != nil {
			weights.EtaAuthority = *s.Authority
		}
		if s.Len != nil {
			weights.DeltaLen = *s.Len
		}
	}

	// Skip expensive WarmInference when inference is disabled (--max-hops=-1)
	skipWarm := *maxHops < 0

	res, err := bootstrap.Run(ctx, bootstrap.Options{
		DBPath:          conf.DBPath,
		StoplistPath:    conf.Stoplist,
		DictPath:        conf.Dict,
		BaseDictPath:    *baseDictPath,
		TaxonomyPath:    conf.Taxonomy,
		RulesPath:       conf.Rules,
		SimpleInference: *simpleInference,
		Weights:         weights,
		SkipWarm:        skipWarm,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	// Build search options from flags
	opts := searchOpts{
		conf:    &conf,
		mode:    korel.SearchMode(*searchMode),
		entity:  *entity,
		maxHops: *maxHops,
		rewrite: *rewrite,
		format:  korel.OutputFormat(*format),
	}
	if *sinceStr != "" {
		t, err := parseTime(*sinceStr)
		if err != nil {
			log.Fatalf("invalid --since: %v", err)
		}
		opts.since = t
	}
	if *untilStr != "" {
		t, err := parseTime(*untilStr)
		if err != nil {
			log.Fatalf("invalid --until: %v", err)
		}
		opts.until = t
	}
	if *relationsStr != "" {
		for _, r := range strings.Split(*relationsStr, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				opts.relations = append(opts.relations, r)
			}
		}
	}

	// One-shot query mode
	if *query != "" {
		if err := executeQuery(ctx, res.Engine, *query, conf.TopK, opts); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Interactive mode
	fmt.Println("===========================================")
	fmt.Println("  Korel Search")
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

		q := strings.TrimSpace(scanner.Text())
		if q == "" {
			continue
		}

		if err := executeQuery(ctx, res.Engine, q, conf.TopK, opts); err != nil {
			fmt.Println("Error:", err)
		}
	}

	fmt.Println("\nGoodbye!")
}

// searchOpts bundles CLI-level search options passed to executeQuery.
type searchOpts struct {
	conf      *searchConfig
	mode      korel.SearchMode
	since     time.Time
	until     time.Time
	entity    string
	maxHops   int
	relations []string
	rewrite   bool
	format    korel.OutputFormat
}

func executeQuery(ctx context.Context, engine *korel.Korel, query string, topK int, opts searchOpts) error {
	req := korel.SearchRequest{
		Query:         query,
		TopK:          topK,
		Now:           time.Now(),
		Mode:          opts.mode,
		Since:         opts.since,
		Until:         opts.until,
		Entity:        opts.entity,
		MaxHops:       opts.maxHops,
		Relations:     opts.relations,
		EnableRewrite: opts.rewrite,
		Format:        opts.format,
	}

	result, err := engine.Search(ctx, req)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	// Print detected intent and rewrite info
	if result.Intent != "" {
		fmt.Printf("Intent: %s\n", result.Intent)
	}
	if result.Rewritten != "" {
		fmt.Printf("Rewritten: %s\n", result.Rewritten)
	}

	// If a formatted output was requested, print it and return
	if result.Formatted != "" {
		fmt.Println(result.Formatted)
		return nil
	}

	if len(result.Cards) == 0 {
		fmt.Println("No results found.")
		fmt.Println()
		return nil
	}

	for i, card := range result.Cards {
		fmt.Printf("\n--- Card %d: %s ---\n", i+1, card.Title)
		for _, bullet := range card.Bullets {
			fmt.Println("  \u2022", bullet)
		}

		fmt.Println("\nSources:")
		for _, src := range card.Sources {
			fmt.Printf("  - %s (%s)\n", src.URL, src.Time.Format("2006-01-02"))
		}

		fmt.Println("\nScore Breakdown:")
		for k, v := range card.ScoreBreakdown {
			fmt.Printf("  %s: %.2f\n", k, v)
		}

		// Evidence quality
		if i < len(result.Evidence) {
			ev := result.Evidence[i]
			fmt.Printf("\nEvidence: freshness=%.2f corroboration=%.2f authority=%.2f overall=%.2f\n",
				ev.Freshness, ev.Corroboration, ev.Authority, ev.Overall)
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
				fmt.Printf("    %s \u2194 %s (PMI: %.2f)\n", p.TokenA, p.TokenB, p.PMI)
			}
		}
		if len(card.Explain.InferencePaths) > 0 {
			fmt.Println("  Inference chains:")
			for _, ip := range card.Explain.InferencePaths {
				fmt.Printf("    %s \u2192 %s: %s\n", ip.From, ip.To, strings.Join(ip.Steps, " \u2192 "))
			}
		}
		fmt.Println()
	}

	// LLM summarization if configured
	conf := opts.conf
	if conf != nil && conf.LLM.BaseURL != "" && conf.LLM.Model != "" {
		llmClient := &llm.Client{
			BaseURL: conf.LLM.BaseURL,
			Model:   conf.LLM.Model,
			APIKey:  conf.LLM.APIKey,
		}
		summary, err := llmClient.Summarize(ctx, query, result.Cards)
		if err != nil {
			return fmt.Errorf("llm summarize: %w", err)
		}
		fmt.Println("LLM Summary:")
		fmt.Println(summary)
	}

	return nil
}
