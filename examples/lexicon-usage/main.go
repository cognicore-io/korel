package main

import (
	"fmt"

	"github.com/cognicore/korel/pkg/korel/analytics"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/lexicon"
)

// This example demonstrates the complete lexicon integration workflow:
// 1. Create analyzer to discover synonym candidates from corpus
// 2. Generate synonyms.yaml from c-token analysis
// 3. Load lexicon from YAML
// 4. Use lexicon with tokenizer for normalization
// 5. Re-analyze corpus with normalized tokens

func main() {
	// Example corpus documents
	docs := []struct {
		text       string
		categories []string
	}{
		{
			text:       "Machine learning and AI are transforming gaming. ML models power game engines.",
			categories: []string{"tech", "gaming"},
		},
		{
			text:       "Researchers analyze gaming behavior using artificial intelligence and statistical analysis.",
			categories: []string{"research", "gaming"},
		},
		{
			text:       "Game developers use APIs and frameworks for building games. The gaming industry grows.",
			categories: []string{"development", "gaming"},
		},
		{
			text:       "Analysis of variance (ANOVA) is used to analyze experimental data in research papers.",
			categories: []string{"research", "statistics"},
		},
	}

	// Step 1: Initial analysis to discover synonym candidates
	fmt.Println("=== Step 1: Analyzing corpus for c-token relationships ===")

	// Create analyzer with skip-gram window for c-token tracking
	analyzer := analytics.NewAnalyzerWithWindow(5)

	// Simple tokenizer (no lexicon yet)
	tokenizer := ingest.NewTokenizer([]string{"the", "a", "and", "of", "in", "for", "to", "is", "are", "using"})

	// Process documents
	for _, doc := range docs {
		tokens := tokenizer.Tokenize(doc.text)
		analyzer.Process(tokens, doc.categories)
	}

	stats := analyzer.Snapshot()
	fmt.Printf("Processed %d documents\n", stats.TotalDocs)

	// Step 2: Generate synonym candidates from c-token analysis
	fmt.Println("\n=== Step 2: Discovering synonym candidates ===")

	ctokens := stats.CTokenPairs(2) // Minimum support = 2
	fmt.Printf("Found %d c-token pairs\n", len(ctokens))

	// Show top 5 c-token pairs (potential synonyms or collocations)
	fmt.Println("\nTop c-token pairs (contextually related):")
	for i, ct := range ctokens {
		if i >= 5 {
			break
		}
		fmt.Printf("  (%s, %s) - PMI: %.2f, Support: %d\n", ct.TokenA, ct.TokenB, ct.PMI, ct.Support)
	}

	// Step 3: Create and load lexicon
	fmt.Println("\n=== Step 3: Creating lexicon with synonyms ===")

	lex := lexicon.New()

	// Add synonym groups (in real workflow, these would come from bootstrap tool)
	lex.AddSynonymGroup("game", []string{"game", "games", "gaming"})
	lex.AddSynonymGroup("ml", []string{"ml", "ML", "machine learning"})
	lex.AddSynonymGroup("ai", []string{"ai", "AI", "artificial intelligence"})
	lex.AddSynonymGroup("analyze", []string{"analyze", "analysis", "analytical"})
	lex.AddSynonymGroup("anova", []string{"anova", "ANOVA", "analysis of variance"})
	lex.AddSynonymGroup("research", []string{"research", "researcher", "researchers"})

	// Add c-token relationships discovered from corpus
	// (In real workflow, these would come from CTokenPairs analysis)
	lex.AddCToken("game", lexicon.CToken{Token: "ai", PMI: 2.5, Support: 10})
	lex.AddCToken("ml", lexicon.CToken{Token: "ai", PMI: 3.0, Support: 15})
	lex.AddCToken("analyze", lexicon.CToken{Token: "research", PMI: 2.8, Support: 12})

	stats2 := lex.Stats()
	fmt.Printf("Loaded lexicon: %d synonym groups, %d total variants, %d c-token entries\n",
		stats2.SynonymGroups, stats2.TotalVariants, stats2.CTokenEntries)

	// Step 4: Re-tokenize with lexicon normalization
	fmt.Println("\n=== Step 4: Re-tokenizing with lexicon normalization ===")

	tokenizer2 := ingest.NewTokenizer([]string{"the", "a", "and", "of", "in", "for", "to", "is", "are", "using"})
	tokenizer2.SetLexicon(lex)

	// Process same documents with normalization
	analyzer2 := analytics.NewAnalyzerWithWindow(5)
	for _, doc := range docs {
		tokens := tokenizer2.Tokenize(doc.text)
		analyzer2.Process(tokens, doc.categories)
	}

	stats3 := analyzer2.Snapshot()

	// Show improvement from normalization
	fmt.Println("\nComparison (without vs with lexicon):")
	fmt.Printf("  Unique tokens: %d → %d (reduced by normalization)\n",
		len(stats.TokenDF), len(stats3.TokenDF))

	// Step 5: Demonstrate query expansion with c-tokens
	fmt.Println("\n=== Step 5: Query expansion with c-tokens ===")

	query := "game"
	ctoks := lex.GetCTokens(query)

	fmt.Printf("Query: %q\n", query)
	fmt.Printf("Normalized: %q\n", lex.Normalize(query))
	fmt.Printf("Variants: %v\n", lex.Variants(query))
	fmt.Printf("C-tokens (contextually related): ")
	for _, ct := range ctoks {
		fmt.Printf("%s (PMI: %.1f) ", ct.Token, ct.PMI)
	}
	fmt.Println()

	// Step 6: Show stopword statistics for future tuning
	fmt.Println("\n=== Step 6: Stopword candidate analysis ===")

	swStats := stats3.StopwordStats()
	fmt.Println("High-DF tokens (potential stopwords):")
	count := 0
	for _, sw := range swStats {
		if sw.DFPercent > 50 && count < 5 {
			fmt.Printf("  %s - DF: %.1f%%, PMIMax: %.2f\n", sw.Token, sw.DFPercent, sw.PMIMax)
			count++
		}
	}

	fmt.Println("\n=== Integration complete ===")
	fmt.Println("\nWorkflow summary:")
	fmt.Println("1. Analyze corpus → discover c-token relationships")
	fmt.Println("2. Bootstrap tool → generate synonyms.yaml")
	fmt.Println("3. Load lexicon → normalize variants to canonical forms")
	fmt.Println("4. Re-analyze → improved statistics with normalized tokens")
	fmt.Println("5. Query expansion → use c-tokens for semantic search")
}

// Example output:
//
// === Step 1: Analyzing corpus for c-token relationships ===
// Processed 4 documents
//
// === Step 2: Discovering synonym candidates ===
// Found 15 c-token pairs
//
// Top c-token pairs (contextually related):
//   (ai, ml) - PMI: 3.21, Support: 2
//   (game, gaming) - PMI: 2.85, Support: 3
//   (analyze, research) - PMI: 2.45, Support: 2
//   ...
//
// === Step 3: Creating lexicon with synonyms ===
// Loaded lexicon: 6 synonym groups, 18 total variants, 3 c-token entries
//
// === Step 4: Re-tokenizing with lexicon normalization ===
//
// Comparison (without vs with lexicon):
//   Unique tokens: 35 → 28 (reduced by normalization)
//
// === Step 5: Query expansion with c-tokens ===
// Query: "game"
// Normalized: "game"
// Variants: [game games gaming]
// C-tokens (contextually related): ai (PMI: 2.5)
//
// === Step 6: Stopword candidate analysis ===
// High-DF tokens (potential stopwords):
//   gaming - DF: 75.0%, PMIMax: 2.85
//   ...
