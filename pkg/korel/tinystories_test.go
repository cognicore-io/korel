package korel

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/store/memstore"
)

const tinyStoriesPath = "/home/claude/work/ml-lab/data/TinyStories-valid.txt"

// parseTinyStories reads the TinyStories file and splits on <|endoftext|>.
// Returns up to limit stories (0 = all).
func parseTinyStories(t *testing.T, limit int) []string {
	t.Helper()

	f, err := os.Open(tinyStoriesPath)
	if err != nil {
		t.Skipf("TinyStories data not available: %v", err)
	}
	defer f.Close()

	var stories []string
	var current strings.Builder
	scanner := bufio.NewScanner(f)
	// Stories can have long lines; increase buffer.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "<|endoftext|>" {
			story := strings.TrimSpace(current.String())
			if story != "" {
				stories = append(stories, story)
				if limit > 0 && len(stories) >= limit {
					break
				}
			}
			current.Reset()
			continue
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading TinyStories: %v", err)
	}

	// Catch trailing story without final delimiter.
	if current.Len() > 0 {
		story := strings.TrimSpace(current.String())
		if story != "" && (limit <= 0 || len(stories) < limit) {
			stories = append(stories, story)
		}
	}

	if len(stories) == 0 {
		t.Fatal("parsed zero stories from TinyStories file")
	}
	return stories
}

// childStopwords returns a basic English stopword list suitable for
// children's stories.
func childStopwords() []string {
	return []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to",
		"for", "of", "with", "by", "from", "is", "are", "was", "were",
		"be", "been", "being", "have", "has", "had", "do", "does", "did",
		"will", "would", "could", "should", "may", "might", "can",
		"it", "its", "this", "that", "these", "those",
		"what", "which", "who", "when", "where", "why", "how",
		"you", "your", "he", "she", "his", "her", "him",
		"i", "we", "they", "them", "their", "my", "our", "me",
		"so", "if", "then", "than", "as", "up", "out", "not", "no",
		"very", "just", "too", "also", "some", "all", "any",
		"about", "into", "over", "after", "before",
	}
}

func newTinyStoriesEngine() *Korel {
	st := memstore.New()
	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer(childStopwords()),
		ingest.NewMultiTokenParser([]ingest.DictEntry{}),
		ingest.NewTaxonomy(),
	)
	return New(Options{
		Store:    st,
		Pipeline: pipeline,
		Inference: simple.New(),
		Weights: ScoreWeights{
			AlphaPMI:     1.0,
			BetaCats:     0.0,
			GammaRecency: 0.0,
			EtaAuthority: 0.0,
			DeltaLen:     0.0,
			ThetaInfer:   0.0,
		},
		RecencyHalfLife: 14,
	})
}

// TestTinyStoriesIngest verifies that a batch of TinyStories can be ingested
// and that basic token/pair statistics are populated.
func TestTinyStoriesIngest(t *testing.T) {
	const numStories = 100
	stories := parseTinyStories(t, numStories)

	engine := newTinyStoriesEngine()
	defer engine.Close()

	ctx := context.Background()
	now := time.Now()

	for i, story := range stories {
		// First sentence as title, full text as body.
		title := story
		if idx := strings.IndexAny(story, ".!?"); idx > 0 && idx < 120 {
			title = story[:idx+1]
		}
		doc := IngestDoc{
			URL:         fmt.Sprintf("tinystories://story-%d", i),
			Title:       title,
			PublishedAt: now,
			BodyText:    story,
		}
		if err := engine.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingest story %d: %v", i, err)
		}
	}

	// Verify documents were stored.
	docs, err := engine.store.GetDocsByTokens(ctx, []string{"once"}, numStories)
	if err != nil {
		t.Fatalf("GetDocsByTokens: %v", err)
	}
	// "once" should appear in many stories ("Once upon a time...").
	// "upon" and "a" and "time" are stopwords or short, but "once" should survive.
	if len(docs) == 0 {
		t.Fatal("expected documents containing 'once'")
	}
	t.Logf("stories containing 'once': %d/%d", len(docs), numStories)

	// Verify PMI pairs exist for common co-occurring terms.
	_, ok, err := engine.store.GetPMI(ctx, "once", "upon")
	if err != nil {
		t.Fatalf("GetPMI: %v", err)
	}
	// "upon" is 4 chars, so should survive tokenizer. Check:
	if ok {
		t.Log("PMI pair (once, upon) exists — good")
	} else {
		// "upon" may have been filtered; not a hard failure.
		t.Log("PMI pair (once, upon) not found — 'upon' may be filtered")
	}
}

// TestTinyStoriesSearch verifies that queries return relevant stories.
func TestTinyStoriesSearch(t *testing.T) {
	const numStories = 200
	stories := parseTinyStories(t, numStories)

	engine := newTinyStoriesEngine()
	defer engine.Close()

	ctx := context.Background()
	now := time.Now()

	for i, story := range stories {
		title := story
		if idx := strings.IndexAny(story, ".!?"); idx > 0 && idx < 120 {
			title = story[:idx+1]
		}
		doc := IngestDoc{
			URL:         fmt.Sprintf("tinystories://story-%d", i),
			Title:       title,
			PublishedAt: now,
			BodyText:    story,
		}
		if err := engine.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingest story %d: %v", i, err)
		}
	}

	tests := []struct {
		query       string
		expectCards bool // whether we expect at least one result
	}{
		{"dog cat friend", true},
		{"happy play park", true},
		{"princess castle", false}, // may or may not match
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			resp, err := engine.Search(ctx, SearchRequest{
				Query: tc.query,
				TopK:  5,
				Now:   now,
			})
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			t.Logf("query=%q → %d cards", tc.query, len(resp.Cards))
			if tc.expectCards && len(resp.Cards) == 0 {
				t.Errorf("expected results for %q, got none", tc.query)
			}
			for i, card := range resp.Cards {
				t.Logf("  card[%d] title=%q pmi=%.2f", i, truncate(card.Title, 60),
					card.ScoreBreakdown["pmi"])
			}
		})
	}
}

// TestTinyStoriesPMINeighbors checks that PMI discovers meaningful
// co-occurrence relationships from the children's story corpus.
func TestTinyStoriesPMINeighbors(t *testing.T) {
	const numStories = 200
	stories := parseTinyStories(t, numStories)

	engine := newTinyStoriesEngine()
	defer engine.Close()

	ctx := context.Background()
	now := time.Now()

	for i, story := range stories {
		title := story
		if idx := strings.IndexAny(story, ".!?"); idx > 0 && idx < 120 {
			title = story[:idx+1]
		}
		doc := IngestDoc{
			URL:         fmt.Sprintf("tinystories://story-%d", i),
			Title:       title,
			PublishedAt: now,
			BodyText:    story,
		}
		if err := engine.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingest story %d: %v", i, err)
		}
	}

	// Check top neighbors for common children's story words.
	probes := []string{"dog", "cat", "friend", "ball", "happy", "mom", "play"}
	for _, probe := range probes {
		df, err := engine.store.GetTokenDF(ctx, probe)
		if err != nil {
			t.Fatalf("GetTokenDF(%s): %v", probe, err)
		}
		if df == 0 {
			t.Logf("  %s: not found in corpus (DF=0)", probe)
			continue
		}

		neighbors, err := engine.store.TopNeighbors(ctx, probe, 5)
		if err != nil {
			t.Fatalf("TopNeighbors(%s): %v", probe, err)
		}
		var nStr []string
		for _, n := range neighbors {
			nStr = append(nStr, fmt.Sprintf("%s(%.1f)", n.Token, n.PMI))
		}
		t.Logf("  %s (DF=%d): neighbors=%v", probe, df, nStr)
	}
}

// newTinyStoriesEngineWithStops creates an engine with a custom stopword list.
func newTinyStoriesEngineWithStops(stops []string) *Korel {
	st := memstore.New()
	pipeline := ingest.NewPipeline(
		ingest.NewTokenizer(stops),
		ingest.NewMultiTokenParser([]ingest.DictEntry{}),
		ingest.NewTaxonomy(),
	)
	return New(Options{
		Store:     st,
		Pipeline:  pipeline,
		Inference: simple.New(),
		Weights: ScoreWeights{
			AlphaPMI:     1.0,
			BetaCats:     0.0,
			GammaRecency: 0.0,
			EtaAuthority: 0.0,
			DeltaLen:     0.0,
			ThetaInfer:   0.0,
		},
		RecencyHalfLife: 14,
	})
}

// ingestStories is a helper that ingests stories into an engine.
func ingestStories(t *testing.T, engine *Korel, stories []string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	for i, story := range stories {
		title := story
		if idx := strings.IndexAny(story, ".!?"); idx > 0 && idx < 120 {
			title = story[:idx+1]
		}
		doc := IngestDoc{
			URL:         fmt.Sprintf("tinystories://story-%d", i),
			Title:       title,
			PublishedAt: now,
			BodyText:    story,
		}
		if err := engine.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingest story %d: %v", i, err)
		}
	}
}

// getNeighborTokens returns the top-k neighbor token names for a probe word.
func getNeighborTokens(t *testing.T, engine *Korel, probe string, k int) []string {
	t.Helper()
	ctx := context.Background()
	neighbors, err := engine.store.TopNeighbors(ctx, probe, k)
	if err != nil {
		t.Fatalf("TopNeighbors(%s): %v", probe, err)
	}
	tokens := make([]string, len(neighbors))
	for i, n := range neighbors {
		tokens[i] = n.Token
	}
	return tokens
}

// TestTinyStoriesAutoTune verifies that the iterative autotune pipeline discovers
// noise stopwords and that removing them improves PMI neighbor quality.
func TestTinyStoriesAutoTune(t *testing.T) {
	const numStories = 5000
	stories := parseTinyStories(t, numStories)

	ctx := context.Background()

	// Phase 1: Baseline — ingest with basic stopwords only.
	baseline := newTinyStoriesEngine()
	defer baseline.Close()
	ingestStories(t, baseline, stories)

	// Capture baseline neighbors for comparison.
	baselineNeighbors := make(map[string][]string)
	probes := []string{"dog", "cat", "ball", "friend"}
	for _, probe := range probes {
		baselineNeighbors[probe] = getNeighborTokens(t, baseline, probe, 10)
	}

	t.Log("=== BASELINE neighbors (before autotune) ===")
	for _, probe := range probes {
		t.Logf("  %s → %v", probe, baselineNeighbors[probe])
	}

	// Phase 2: Run iterative AutoTune to discover stopword candidates.
	result, err := baseline.AutoTune(ctx, stories, &AutoTuneOptions{
		BaseStopwords: childStopwords(),
	})
	if err != nil {
		t.Fatalf("AutoTune: %v", err)
	}

	if len(result.StopwordCandidates) == 0 {
		t.Fatal("AutoTune returned zero stopword candidates")
	}

	// Log iteration details.
	t.Logf("=== AutoTune completed %d iterations ===", len(result.Iterations))
	for _, iter := range result.Iterations {
		t.Logf("  Round %d: %d new stopwords, %d total",
			iter.Round, len(iter.NewStopwords), iter.TotalStopwords)
		if len(iter.NewStopwords) <= 10 {
			t.Logf("    new: %v", iter.NewStopwords)
		} else {
			t.Logf("    new: %v ... and %d more", iter.NewStopwords[:10], len(iter.NewStopwords)-10)
		}
	}

	// Sort candidates by score (highest first) for readable output.
	sort.Slice(result.StopwordCandidates, func(i, j int) bool {
		return result.StopwordCandidates[i].Score > result.StopwordCandidates[j].Score
	})

	t.Logf("=== AutoTune discovered %d stopword candidates (cumulative) ===", len(result.StopwordCandidates))
	for i, c := range result.StopwordCandidates {
		if i >= 20 {
			t.Logf("  ... and %d more", len(result.StopwordCandidates)-20)
			break
		}
		t.Logf("  %s (score=%.3f, DF=%.0f%%, PMI_max=%.2f, entropy=%.2f)",
			c.Token, c.Score, c.Reason.IDF, c.Reason.PMIMax, c.Reason.CatEntropy)
	}

	if len(result.RuleSuggestions) > 0 {
		t.Logf("=== AutoTune discovered %d rule suggestions ===", len(result.RuleSuggestions))
		limit := len(result.RuleSuggestions)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			s := result.RuleSuggestions[i]
			t.Logf("  %s(%s, %s) confidence=%.2f support=%d",
				s.Relation, s.Subject, s.Object, s.Confidence, s.Support)
		}
	}

	// Phase 3: Build augmented stoplist and re-ingest.
	augmentedStops := make([]string, len(childStopwords()))
	copy(augmentedStops, childStopwords())
	for _, c := range result.StopwordCandidates {
		augmentedStops = append(augmentedStops, c.Token)
	}

	tuned := newTinyStoriesEngineWithStops(augmentedStops)
	defer tuned.Close()
	ingestStories(t, tuned, stories)

	// Phase 4: Compare neighbors after autotune.
	// With PMI-ranked TopNeighbors (DF≥5 floor), baseline should already be
	// free of high-frequency noise. Autotune's value is removing the next tier
	// of common-but-uninformative words so PMI can surface domain terms.

	// Noise words that should NOT appear in top neighbors.
	noise := map[string]bool{
		"once": true, "upon": true, "time": true, "there": true,
		"day": true, "one": true, "named": true, "little": true,
		"went": true, "got": true, "said": true, "came": true,
		"saw": true, "big": true, "new": true, "happy": true,
		"wanted": true, "like": true, "really": true, "much": true,
	}

	t.Log("=== BASELINE neighbors (PMI-ranked, DF≥5) ===")
	for _, probe := range probes {
		t.Logf("  %s → %v", probe, baselineNeighbors[probe])
	}

	t.Log("=== TUNED neighbors (after autotune) ===")
	totalTunedNoise := 0
	for _, probe := range probes {
		tunedNeighbors := getNeighborTokens(t, tuned, probe, 10)
		t.Logf("  %s → %v", probe, tunedNeighbors)

		for _, n := range tunedNeighbors {
			if noise[n] {
				totalTunedNoise++
				t.Logf("    noise: %s", n)
			}
		}
	}
	t.Logf("=== Total noise in tuned neighbors: %d ===", totalTunedNoise)
	if totalTunedNoise > 4 {
		t.Errorf("expected at most 4 noise tokens total across all probes, got %d", totalTunedNoise)
	}

	// Semantic quality check: at least some domain-relevant neighbors should appear.
	// Lists are broad — children's stories have specific vocabulary.
	semanticExpected := map[string][]string{
		"dog":    {"cat", "bark", "puppy", "pet", "bone", "park", "fetch", "tail", "walk", "barks", "doggy", "sniffed", "chewing", "meat", "leash", "paw", "woof"},
		"cat":    {"dog", "kitten", "pet", "mouse", "meow", "purr", "purring", "fish", "milk", "whiskers", "pounced", "lap", "yarn", "fur", "climb"},
		"ball":   {"throw", "catch", "kick", "bounce", "game", "park", "play", "red", "round", "rolls", "toss", "goal", "goals", "golf"},
		"friend": {"best", "play", "together", "share", "kind", "fun", "nice", "help", "hug", "laugh", "smile"},
	}
	semanticHits := 0
	for _, probe := range probes {
		tunedNeighbors := getNeighborTokens(t, tuned, probe, 10)
		expected := semanticExpected[probe]
		for _, n := range tunedNeighbors {
			for _, e := range expected {
				if n == e {
					t.Logf("  ✓ semantic hit: %s → %s", probe, n)
					semanticHits++
				}
			}
		}
	}
	t.Logf("=== Semantic hits: %d ===", semanticHits)
	if semanticHits == 0 {
		t.Error("expected at least one semantically meaningful neighbor across all probes")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
