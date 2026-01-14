package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/cognicore/korel/internal/llm"
	"github.com/cognicore/korel/internal/rss"
	"github.com/cognicore/korel/pkg/korel/analytics"
	reviewer "github.com/cognicore/korel/pkg/korel/autotune/review/llm"
	"github.com/cognicore/korel/pkg/korel/autotune/stopwords"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/stoplist"
)

var (
	urlPattern    = regexp.MustCompile(`https?://[^\s\]]+`)
	sourcePattern = regexp.MustCompile(`\[Source:\s*[^\]]*\]`)
	authorPattern = regexp.MustCompile(`(?i)\bauthors?:\s*[^\n]+`)
)

// iterationConfig holds parameters for iterative analysis.
type iterationConfig struct {
	baseStoplistPath string
	noBaseStoplist   bool
	stopLimit        int
	maxIterations    int
	llmBase          string
	llmModel         string
	llmKey           string
}

// iterationResult holds the final results after convergence.
type iterationResult struct {
	stats         analytics.Stats
	allStopwords  []string
	iterationLog  []iterationStep
}

// iterationStep tracks progress of each iteration.
type iterationStep struct {
	Iteration      int      `json:"iteration"`
	StopwordsAdded int      `json:"stopwords_added"`
	NewStopwords   []string `json:"new_stopwords"`
	TotalStopwords int      `json:"total_stopwords"`
}

// runIterativeAnalysis performs iterative stopword discovery and corpus re-analysis.
//
// Algorithm:
// 1. Start with base stopwords from file (e.g., English common words)
// 2. Tokenize corpus with current stopwords
// 3. Analyze statistics and discover high-DF/low-PMI candidates
// 4. Add discovered stopwords to base list
// 5. Repeat from step 2 until:
//    - No new stopwords discovered (converged), OR
//    - Maximum iterations reached
//
// Why iterative?
// - First pass may miss stopwords that only become visible after removing other noise
// - Example: "can be" bigram only disappears after "can" AND "be" both filtered
// - Converges quickly (typically 2-3 iterations for 500+ docs)
func runIterativeAnalysis(ctx context.Context, items []rss.Item, cfg iterationConfig) iterationResult {
	// Load base stopwords from file (or start cold if --no-base-stoplist)
	var currentStopwords []string
	if cfg.noBaseStoplist {
		log.Printf("Starting with empty stoplist (--no-base-stoplist enabled)")
	} else {
		currentStopwords = loadStopwordsFromFile(cfg.baseStoplistPath)
		if len(currentStopwords) == 0 {
			log.Printf("WARNING: Base stoplist %s is empty or missing. For non-English corpora, provide a language-specific base stoplist or use --no-base-stoplist for cold-start", cfg.baseStoplistPath)
		} else {
			log.Printf("Loaded %d base stopwords from %s", len(currentStopwords), cfg.baseStoplistPath)
		}
	}

	var iterations []iterationStep
	var finalStats analytics.Stats

	for i := 0; i < cfg.maxIterations; i++ {
		log.Printf("=== Iteration %d/%d: analyzing with %d stopwords ===", i+1, cfg.maxIterations, len(currentStopwords))

		// Create pipeline with current stopwords
		pipeline := ingest.NewPipeline(
			ingest.NewTokenizer(currentStopwords),
			ingest.NewMultiTokenParser(nil),
			ingest.NewTaxonomy(),
		)

		// Analyze corpus
		analyzer := analytics.NewAnalyzer()
		for _, item := range items {
			normalizedText := cleanText(item.Body)
			processed := pipeline.Process(normalizedText)
			analyzer.Process(processed.Tokens, processed.Categories)
		}
		finalStats = analyzer.Snapshot()

		// Discover new stopwords
		newStops := generateStopwords(ctx, finalStats, cfg.baseStoplistPath, cfg.noBaseStoplist, cfg.stopLimit, cfg.llmBase, cfg.llmModel, cfg.llmKey)

		// Remove stopwords already in our list
		var actuallyNew []string
		existingSet := make(map[string]bool)
		for _, s := range currentStopwords {
			existingSet[s] = true
		}
		for _, s := range newStops {
			if !existingSet[s] {
				actuallyNew = append(actuallyNew, s)
			}
		}

		iterations = append(iterations, iterationStep{
			Iteration:      i + 1,
			StopwordsAdded: len(actuallyNew),
			NewStopwords:   actuallyNew,
			TotalStopwords: len(currentStopwords) + len(actuallyNew),
		})

		if len(actuallyNew) == 0 {
			log.Printf("Converged! No new stopwords discovered in iteration %d.", i+1)
			break
		}

		log.Printf("Discovered %d new stopwords: %v", len(actuallyNew), actuallyNew)
		currentStopwords = append(currentStopwords, actuallyNew...)
	}

	return iterationResult{
		stats:        finalStats,
		allStopwords: currentStopwords,
		iterationLog: iterations,
	}
}

// loadStopwordsFromFile loads stopwords from YAML file.
func loadStopwordsFromFile(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var sl struct {
		Terms []string `yaml:"terms"`
	}
	if err := yaml.Unmarshal(data, &sl); err != nil {
		return nil
	}
	return sl.Terms
}

// loadBlacklist loads generic terms to exclude from taxonomy from YAML file.
func loadBlacklist(path string) map[string]struct{} {
	result := make(map[string]struct{})
	if path == "" {
		return result
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("WARNING: Failed to load taxonomy blacklist from %s: %v", path, err)
		return result
	}
	var bl struct {
		Terms []string `yaml:"terms"`
	}
	if err := yaml.Unmarshal(data, &bl); err != nil {
		log.Printf("WARNING: Failed to parse taxonomy blacklist from %s: %v", path, err)
		return result
	}
	for _, term := range bl.Terms {
		result[strings.ToLower(term)] = struct{}{}
	}
	return result
}

func main() {
	var (
		input            = flag.String("input", "", "Path to raw JSONL corpus (required)")
		domain           = flag.String("domain", "", "Domain identifier (required)")
		outDir           = flag.String("output", "", "Output directory for generated configs (required)")
		baseStoplist     = flag.String("base-stoplist", "configs/stopwords-en.yaml", "Base stopword list to start from (default: English)")
		noBaseStoplist   = flag.Bool("no-base-stoplist", false, "Start from empty stoplist (true cold-start, overrides --base-stoplist)")
		stopLimit        = flag.Int("stop-limit", 25, "Number of stopword suggestions per iteration")
		pairLimit        = flag.Int("pair-limit", 10, "Number of multi-token suggestions")
		pairMinSupport   = flag.Int("pair-min-support", 3, "Minimum support for multi-token pairs")
		pairMinPMI       = flag.Float64("pair-min-pmi", 1.0, "Minimum PMI for multi-token pairs")
		taxonomyLimit    = flag.Int("taxonomy-limit", 150, "Maximum keywords per taxonomy category")
		taxonomyMinSupport = flag.Int("taxonomy-min-support", 3, "Minimum support for taxonomy clustering")
		taxonomyMinPMI   = flag.Float64("taxonomy-min-pmi", 0.8, "Minimum PMI for taxonomy clustering")
		taxonomyBlacklist = flag.String("taxonomy-blacklist", "", "YAML file with generic terms to exclude from taxonomy (optional)")
		requireTaxonomyLLM = flag.Bool("require-taxonomy-llm", false, "Fail if clustering fails instead of generating fallback taxonomy")
		synonymLimit     = flag.Int("synonym-limit", 20, "Maximum synonym candidates to suggest")
		synonymMinSupport = flag.Int("synonym-min-support", 3, "Minimum support for synonym candidates")
		synonymMinPMI    = flag.Float64("synonym-min-pmi", 1.5, "Minimum c-token PMI for synonym candidates")
		maxIterations    = flag.Int("iterations", 2, "Maximum refinement iterations (0=no iteration, 1+=iterative stopword discovery)")
		llmBase          = flag.String("llm-base", "", "Optional LLM reviewer base URL")
		llmModel         = flag.String("llm-model", "", "Optional LLM reviewer model")
		llmKey           = flag.String("llm-api-key", "", "Optional LLM reviewer API key")
		taxLLMBase       = flag.String("taxonomy-llm-base", "", "Optional taxonomy LLM base URL")
		taxLLMModel      = flag.String("taxonomy-llm-model", "", "Optional taxonomy LLM model")
		taxLLMKey        = flag.String("taxonomy-llm-api-key", "", "Optional taxonomy LLM API key")
	)
	flag.Parse()

	if *input == "" || *domain == "" || *outDir == "" {
		log.Fatal("--input, --domain, and --output are required")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	ctx := context.Background()

	// Load initial documents once
	items, err := rss.LoadFromJSONL(*input)
	if err != nil {
		log.Fatalf("load docs: %v", err)
	}
	if len(items) == 0 {
		log.Fatal("no documents found")
	}

	// Iterative refinement: discover stopwords, re-tokenize, repeat until convergence
	// This improves quality by:
	// 1. Removing stopword noise from token statistics (better DF/PMI)
	// 2. Preventing stopword bigrams like "can be", "has been"
	// 3. Improving taxonomy quality (fewer generic terms)
	result := runIterativeAnalysis(ctx, items, iterationConfig{
		baseStoplistPath: *baseStoplist,
		noBaseStoplist:   *noBaseStoplist,
		stopLimit:        *stopLimit,
		maxIterations:    *maxIterations,
		llmBase:          *llmBase,
		llmModel:         *llmModel,
		llmKey:           *llmKey,
	})

	// Generate final outputs from converged analysis
	stats := result.stats
	suggestions := result.allStopwords
	pairs := filterPairs(stats.TopPairs(*pairLimit*2, *pairMinPMI), int64(*pairMinSupport), limitInt(*pairLimit))
	highDF := topHighDF(stats, 20)
	synonymCandidates := generateSynonymCandidates(stats, int64(*synonymMinSupport), *synonymMinPMI, *synonymLimit)
	taxonomyCategories := generateTaxonomy(ctx, stats, suggestions, *domain, *taxonomyLimit, taxonomyLLMConfig{
		BaseURL: *taxLLMBase,
		Model:   *taxLLMModel,
		APIKey:  *taxLLMKey,
	}, *taxonomyMinSupport, *taxonomyMinPMI, *taxonomyBlacklist, *requireTaxonomyLLM)

	if err := writeStoplist(filepath.Join(*outDir, "stoplist.yaml"), suggestions); err != nil {
		log.Fatalf("write stoplist: %v", err)
	}
	if err := writeSynonyms(filepath.Join(*outDir, "synonyms.yaml"), synonymCandidates); err != nil {
		log.Fatalf("write synonyms: %v", err)
	}
	if len(synonymCandidates) == 0 {
		log.Printf("WARNING: No synonym candidates found. Corpus may be too small or thresholds too high.")
	}
	if err := writeTokens(filepath.Join(*outDir, "tokens.dict"), pairs, taxonomyCategories); err != nil {
		log.Fatalf("write tokens: %v", err)
	}
	if len(pairs) == 0 {
		log.Printf("WARNING: No multi-token phrases found. Corpus may be too small (need 100+ docs for reliable PMI). tokens.dict is empty.")
	}
	if err := writeTaxonomy(filepath.Join(*outDir, "taxonomies.yaml"), taxonomyCategories); err != nil {
		log.Fatalf("write taxonomy: %v", err)
	}
	if err := writeReport(filepath.Join(*outDir, "bootstrap-report.json"), stats, suggestions, pairs, synonymCandidates, highDF, taxonomyCategories, result.iterationLog); err != nil {
		log.Fatalf("write report: %v", err)
	}

	log.Printf("Bootstrap complete after %d iterations", len(result.iterationLog))
	fmt.Printf("Bootstrap configs written to %s\n", *outDir)
}

func generateStopwords(ctx context.Context, stats analytics.Stats, baseStoplistPath string, noBaseStoplist bool, limit int, llmBase, llmModel, llmKey string) []string {
	// Load base stopwords from file if it exists, otherwise use minimal set
	var base []string

	if noBaseStoplist {
		// True cold start - no base stopwords at all
		base = []string{}
	} else {
		if baseStoplistPath != "" {
			if data, err := os.ReadFile(baseStoplistPath); err == nil {
				var stopData struct {
					Terms []string `yaml:"terms"`
				}
				if err := yaml.Unmarshal(data, &stopData); err == nil {
					base = stopData.Terms
				}
			}
		}

		// Fallback to absolute minimum if no file or loading failed
		if len(base) == 0 {
			base = []string{"the", "a", "and", "of"}
		}
	}

	mgr := stoplist.NewManager(base)

	tuner := stopwords.AutoTuner{
		Provider: analytics.NewStopwordStatsProvider(stats),
		Manager:  mgr,
	}

	candidates, err := tuner.Run(ctx)
	if err != nil {
		log.Printf("stopword tuner error: %v", err)
		return base
	}

	if llmBase != "" && llmModel != "" {
		reviewer := &reviewer.Client{Endpoint: llmBase, APIKey: llmKey}
		var filtered []stoplist.Candidate
		for i, cand := range candidates {
			if limit > 0 && i >= limit {
				break
			}
			ok, err := reviewer.Approve(ctx, cand)
			if err != nil {
				log.Printf("reviewer error %s: %v", cand.Token, err)
				continue
			}
			if ok {
				filtered = append(filtered, cand)
			}
		}
		candidates = filtered
	} else if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	tokens := make(map[string]struct{})
	for _, tok := range base {
		tokens[tok] = struct{}{}
	}
	for _, cand := range candidates {
		tokens[cand.Token] = struct{}{}
	}

	list := make([]string, 0, len(tokens))
	for tok := range tokens {
		list = append(list, tok)
	}
	sort.Strings(list)
	return list
}

func writeStoplist(path string, terms []string) error {
	data := struct {
		Terms []string `yaml:"terms"`
	}{Terms: terms}
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

func writeTokens(path string, pairs []analytics.PairStat, taxonomy map[string][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Build reverse lookup: token -> categories
	tokenToCategories := make(map[string][]string)
	for categoryName, keywords := range taxonomy {
		for _, keyword := range keywords {
			tokenToCategories[strings.ToLower(keyword)] = append(tokenToCategories[strings.ToLower(keyword)], categoryName)
		}
	}

	for _, p := range pairs {
		canonical := fmt.Sprintf("%s %s", p.A, p.B)

		// Try to find best matching category for this pair
		category := findBestCategory(p.A, p.B, tokenToCategories)

		line := fmt.Sprintf("%s|%s|%s\n", canonical, canonical, category)
		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

// findBestCategory finds the most appropriate category for a token pair.
// Returns the category if both tokens appear in it, or the first matching category,
// or empty string if no match.
func findBestCategory(tokenA, tokenB string, tokenToCategories map[string][]string) string {
	catsA := tokenToCategories[strings.ToLower(tokenA)]
	catsB := tokenToCategories[strings.ToLower(tokenB)]

	// Prefer categories where both tokens appear
	for _, catA := range catsA {
		for _, catB := range catsB {
			if catA == catB {
				return catA
			}
		}
	}

	// Fall back to category of first token
	if len(catsA) > 0 {
		return catsA[0]
	}

	// Fall back to category of second token
	if len(catsB) > 0 {
		return catsB[0]
	}

	// No category found - leave empty so users know it wasn't classified
	return ""
}

func writeTaxonomy(path string, categories map[string][]string) error {
	// generateTaxonomy should never return empty categories (will log.Fatal if --require-taxonomy-llm set)
	// But if it somehow does, write an empty taxonomy file to avoid silent failures
	data := struct {
		Sectors map[string][]string `yaml:"sectors"`
	}{Sectors: categories}
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

func writeReport(path string, stats analytics.Stats, stopwords []string, pairs []analytics.PairStat, synonyms []synonymCandidate, highDF []highDFEntry, taxonomy map[string][]string, iterationLog []iterationStep) error {
	report := struct {
		GeneratedAt  time.Time            `json:"generated_at"`
		TotalDocs    int64                `json:"total_docs"`
		Iterations   []iterationStep      `json:"iterations"`          // Tracks iterative refinement
		Stopwords    []string             `json:"stopwords"`
		Pairs        []analytics.PairStat `json:"pairs"`
		Synonyms     []synonymCandidate   `json:"synonym_candidates"`  // C-token based synonym suggestions
		HighDF       []highDFEntry        `json:"high_df_tokens"`
		Taxonomy     map[string][]string  `json:"taxonomy"`
	}{
		GeneratedAt:  time.Now(),
		TotalDocs:    stats.TotalDocs,
		Iterations:   iterationLog,
		Stopwords:    stopwords,
		Pairs:        pairs,
		Synonyms:     synonyms,
		HighDF:       highDF,
		Taxonomy:     taxonomy,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func topHighDF(stats analytics.Stats, limit int) []highDFEntry {
	entries := stats.StopwordStats()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DFPercent > entries[j].DFPercent
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]highDFEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, highDFEntry{Token: entry.Token, DFPercent: entry.DFPercent, Entropy: entry.CatEntropy})
	}
	return out
}

func generateTaxonomy(ctx context.Context, stats analytics.Stats, stopwords []string, domain string, limit int, cfg taxonomyLLMConfig, minSupport int, minPMI float64, blacklistPath string, requireLLM bool) map[string][]string {
	stopSet := make(map[string]struct{})
	for _, tok := range stopwords {
		stopSet[tok] = struct{}{}
	}

	// Load generic terms blacklist if provided
	var genericTerms map[string]struct{}
	if blacklistPath != "" {
		genericTerms = loadBlacklist(blacklistPath)
		if len(genericTerms) > 0 {
			log.Printf("Loaded %d generic terms from blacklist: %s", len(genericTerms), blacklistPath)
		}
	}

	clusters := buildClusters(stats, stopSet, int64(minSupport), minPMI)
	if len(clusters) == 0 {
		if requireLLM {
			log.Fatal("FATAL: No semantic clusters found and --require-taxonomy-llm is set. " +
				"Either provide --taxonomy-llm-* flags, increase corpus size (200+ docs), " +
				"or lower thresholds (--taxonomy-min-support, --taxonomy-min-pmi).")
		}
		log.Printf("WARNING: No semantic clusters found. Falling back to high-frequency keywords.")
		log.Printf("For better taxonomy, use --taxonomy-llm-* flags or provide 200+ documents.")
		return map[string][]string{domain + "-fallback": extractKeywords(stats, stopSet, genericTerms, limit)}
	}
	if cfg.BaseURL != "" && cfg.Model != "" {
		if llmCats, err := llmTaxonomy(ctx, clusters, cfg, domain); err == nil && len(llmCats) > 0 {
			return llmCats
		}
		log.Printf("WARNING: LLM taxonomy generation failed, using cluster-based fallback")
	}
	result := make(map[string][]string)
	for idx, cluster := range clusters {
		name := fmt.Sprintf("%s-%d", domain, idx+1)
		result[name] = cluster
	}
	return result
}

func buildClusters(stats analytics.Stats, stopwords map[string]struct{}, minSupport int64, minPMI float64) [][]string {
	pairs := stats.TopPairs(200, minPMI)
	graph := make(map[string]map[string]struct{})
	addEdge := func(a, b string) {
		if graph[a] == nil {
			graph[a] = make(map[string]struct{})
		}
		graph[a][b] = struct{}{}
	}
	for _, p := range pairs {
		if p.Support < minSupport {
			continue
		}
		if _, banned := stopwords[p.A]; banned {
			continue
		}
		if _, banned := stopwords[p.B]; banned {
			continue
		}
		addEdge(p.A, p.B)
		addEdge(p.B, p.A)
	}
	visited := make(map[string]bool)
	var clusters [][]string
	for node := range graph {
		if visited[node] {
			continue
		}
		queue := []string{node}
		visited[node] = true
		var cluster []string
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			cluster = append(cluster, cur)
			for neighbor := range graph[cur] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		if len(cluster) >= 3 {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}

func llmTaxonomy(ctx context.Context, clusters [][]string, cfg taxonomyLLMConfig, domain string) (map[string][]string, error) {
	if cfg.BaseURL == "" || cfg.Model == "" {
		return nil, errors.New("taxonomy llm config incomplete")
	}
	client := &llm.Client{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey}
	payload := struct {
		Domain   string     `json:"domain"`
		Clusters [][]string `json:"clusters"`
	}{Domain: domain, Clusters: clusters}
	body, _ := json.Marshal(payload)
	system := "You are a knowledge taxonomy builder. Return compact JSON with category names and keywords based only on the provided clusters."
	user := fmt.Sprintf("Input: %s\nRespond with JSON: {\"categories\":[{\"name\":string,\"keywords\":[...]},...]}", string(body))
	resp, err := client.Chat(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Categories []struct {
			Name     string   `json:"name"`
			Keywords []string `json:"keywords"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	for _, cat := range parsed.Categories {
		if cat.Name == "" || len(cat.Keywords) == 0 {
			continue
		}
		result[cat.Name] = dedupe(cat.Keywords)
	}
	return result, nil
}

func extractKeywords(stats analytics.Stats, stopwords map[string]struct{}, genericTerms map[string]struct{}, limit int) []string {
	// If no custom blacklist provided, use default English/HN-centric terms
	if len(genericTerms) == 0 {
		log.Printf("WARNING: Using hard-coded English/HN generic terms for taxonomy. " +
			"For other domains, provide --taxonomy-blacklist with domain-specific terms to exclude.")
		genericTerms = map[string]struct{}{
			"new": {}, "first": {}, "second": {}, "third": {}, "thanks": {}, "please": {},
			"hn": {}, "news": {}, "show": {}, "ask": {}, "tell": {}, "launch": {},
			"today": {}, "ago": {}, "via": {}, "nov": {}, "dec": {}, "jan": {}, "feb": {},
			"mar": {}, "apr": {}, "may": {}, "jun": {}, "jul": {}, "aug": {}, "sep": {}, "oct": {},
		}
	}

	entries := stats.StopwordStats()
	seen := make(map[string]struct{})
	var words []string

	for _, entry := range entries {
		tok := strings.ToLower(entry.Token)

		// Skip stopwords
		if _, banned := stopwords[tok]; banned {
			continue
		}

		// Skip generic terms
		if _, generic := genericTerms[tok]; generic {
			continue
		}

		// Skip by DF percentage
		if entry.DFPercent < 2 || entry.DFPercent > 60 {
			continue
		}

		// Clean up token: remove leading/trailing non-alphanumeric
		cleaned := strings.TrimFunc(tok, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})

		// Skip if empty after cleanup, too short, or too long
		if len(cleaned) < 3 || len(cleaned) > 30 {
			continue
		}

		// Skip if mostly numbers
		digitCount := 0
		for _, r := range cleaned {
			if unicode.IsDigit(r) {
				digitCount++
			}
		}
		if float64(digitCount)/float64(len(cleaned)) > 0.5 {
			continue
		}

		// Skip if too many hyphens (likely concatenated junk)
		if strings.Count(cleaned, "-") > 2 {
			continue
		}

		// Skip duplicates
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}

		words = append(words, cleaned)
		if limit > 0 && len(words) >= limit {
			break
		}
	}

	if len(words) == 0 {
		log.Printf("WARNING: No valid taxonomy keywords found. Corpus may be too small or use --taxonomy-llm-* for LLM-generated taxonomy")
		words = []string{"general"}
	}

	return words
}

func dedupe(list []string) []string {
	set := make(map[string]struct{})
	var out []string
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

type highDFEntry struct {
	Token     string  `json:"token"`
	DFPercent float64 `json:"df_percent"`
	Entropy   float64 `json:"entropy"`
}

type taxonomyLLMConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

func limitInt(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func filterPairs(pairs []analytics.PairStat, minSupport int64, limit int) []analytics.PairStat {
	var filtered []analytics.PairStat
	for _, p := range pairs {
		if p.Support < minSupport {
			continue
		}
		filtered = append(filtered, p)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

// cleanText removes common metadata patterns that don't add semantic value:
// - URLs and source attributions (e.g. [Source: https://...])
// - Author attributions (e.g. "Authors: John Doe, Jane Smith")
// This normalization improves token quality for statistical analysis.
func cleanText(text string) string {
	// Strip [Source: URL] markdown blocks (common in news/blog aggregators)
	text = sourcePattern.ReplaceAllString(text, "")
	// Strip author attributions (common in academic papers, research articles)
	text = authorPattern.ReplaceAllString(text, "")
	// Strip remaining URLs
	text = urlPattern.ReplaceAllString(text, "")
	return text
}

// synonymCandidate represents a suggested synonym group for manual review.
type synonymCandidate struct {
	Canonical string   `yaml:"canonical"`
	Variants  []string `yaml:"variants"`
	PMI       float64  `yaml:"pmi"`       // Contextual similarity strength
	Support   int64    `yaml:"support"`   // Co-occurrence frequency
}

// generateSynonymCandidates uses skip-gram c-token statistics to suggest
// synonym groups based on contextual similarity (high PMI co-occurrence).
//
// Algorithm:
// 1. Get c-token pairs from skip-gram window analysis
// 2. Filter by minimum support and PMI thresholds
// 3. Group tokens by shared context (connected components)
// 4. Suggest groups as synonym candidates for manual review
//
// Note: These are candidates, not confirmed synonyms. Manual review recommended
// to distinguish true synonyms from related terms (e.g., "game/gaming" vs "machine/learning").
func generateSynonymCandidates(stats analytics.Stats, minSupport int64, minPMI float64, limit int) []synonymCandidate {
	// Get high-PMI c-token pairs from skip-gram analysis
	ctokens := stats.CTokenPairs(minSupport)

	// Build graph of contextually related tokens
	graph := make(map[string]map[string]analytics.CTokenPair)
	for _, ct := range ctokens {
		if ct.PMI < minPMI {
			continue // Sorted by PMI, so we can stop here
		}

		// Add bidirectional edges
		if graph[ct.TokenA] == nil {
			graph[ct.TokenA] = make(map[string]analytics.CTokenPair)
		}
		if graph[ct.TokenB] == nil {
			graph[ct.TokenB] = make(map[string]analytics.CTokenPair)
		}
		graph[ct.TokenA][ct.TokenB] = ct
		graph[ct.TokenB][ct.TokenA] = ct
	}

	// Find connected components (token groups with shared context)
	visited := make(map[string]bool)
	var candidates []synonymCandidate

	for token := range graph {
		if visited[token] {
			continue
		}

		// BFS to find connected component
		queue := []string{token}
		visited[token] = true
		var group []string
		var maxPMI float64
		var totalSupport int64

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			group = append(group, cur)

			for neighbor, ct := range graph[cur] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
				if ct.PMI > maxPMI {
					maxPMI = ct.PMI
				}
				totalSupport += ct.Support
			}
		}

		// Only suggest groups with 2+ tokens
		if len(group) >= 2 {
			sort.Strings(group)
			candidates = append(candidates, synonymCandidate{
				Canonical: group[0], // First alphabetically as canonical
				Variants:  group[1:],
				PMI:       maxPMI,
				Support:   totalSupport,
			})
		}

		if limit > 0 && len(candidates) >= limit {
			break
		}
	}

	// Sort by PMI (strongest contextual relationships first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].PMI > candidates[j].PMI
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates
}

// writeSynonyms writes synonym candidates to YAML file.
// Output format is compatible with lexicon.LoadFromYAML for easy editing and loading.
func writeSynonyms(path string, candidates []synonymCandidate) error {
	data := struct {
		Synonyms []synonymCandidate `yaml:"synonyms"`
	}{Synonyms: candidates}

	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Add header comment explaining these are candidates for review
	header := []byte("# Synonym candidates generated from corpus c-token analysis\n" +
		"# These are suggestions based on contextual similarity (high PMI co-occurrence)\n" +
		"# IMPORTANT: Review and edit manually to remove false positives\n" +
		"# - Keep true synonyms: game/games/gaming\n" +
		"# - Remove collocations: machine/learning (appear together, not synonyms)\n" +
		"# - Remove unrelated pairs that happen to share context\n\n")

	return os.WriteFile(path, append(header, buf...), 0o644)
}
