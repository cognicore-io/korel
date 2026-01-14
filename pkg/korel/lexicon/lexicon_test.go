package lexicon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLexiconNew(t *testing.T) {
	lex := New()
	if lex == nil {
		t.Fatal("New() returned nil")
	}

	stats := lex.Stats()
	if stats.SynonymGroups != 0 {
		t.Errorf("New lexicon should have 0 synonym groups, got %d", stats.SynonymGroups)
	}
}

func TestLexiconAddSynonymGroup(t *testing.T) {
	lex := New()

	// Add first synonym group
	lex.AddSynonymGroup("game", []string{"game", "games", "gaming", "gamer"})

	// Test normalization
	if got := lex.Normalize("gaming"); got != "game" {
		t.Errorf("Normalize('gaming') = %q, want 'game'", got)
	}
	if got := lex.Normalize("gamer"); got != "game" {
		t.Errorf("Normalize('gamer') = %q, want 'game'", got)
	}
	if got := lex.Normalize("game"); got != "game" {
		t.Errorf("Normalize('game') = %q, want 'game'", got)
	}

	// Test variants
	variants := lex.Variants("gaming")
	if len(variants) != 4 {
		t.Errorf("Variants('gaming') returned %d variants, want 4", len(variants))
	}

	// Verify all expected variants are present
	expected := map[string]bool{"game": false, "games": false, "gaming": false, "gamer": false}
	for _, v := range variants {
		if _, ok := expected[v]; ok {
			expected[v] = true
		}
	}
	for variant, found := range expected {
		if !found {
			t.Errorf("Variants('gaming') missing expected variant %q", variant)
		}
	}
}

func TestLexiconCaseInsensitive(t *testing.T) {
	lex := New()
	lex.AddSynonymGroup("ml", []string{"ml", "ML", "Machine Learning", "machine-learning"})

	// Test normalization with different cases
	tests := []struct {
		input string
		want  string
	}{
		{"ML", "ml"},
		{"ml", "ml"},
		{"Machine Learning", "ml"},
		{"machine learning", "ml"},
		{"MACHINE-LEARNING", "ml"},
	}

	for _, tt := range tests {
		if got := lex.Normalize(tt.input); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLexiconUnknownToken(t *testing.T) {
	lex := New()
	lex.AddSynonymGroup("game", []string{"game", "games"})

	// Unknown token should return itself
	if got := lex.Normalize("unknown"); got != "unknown" {
		t.Errorf("Normalize('unknown') = %q, want 'unknown'", got)
	}

	// Unknown token variants should return singleton
	variants := lex.Variants("unknown")
	if len(variants) != 1 || variants[0] != "unknown" {
		t.Errorf("Variants('unknown') = %v, want ['unknown']", variants)
	}

	// HasSynonyms should return false
	if lex.HasSynonyms("unknown") {
		t.Error("HasSynonyms('unknown') = true, want false")
	}
}

func TestLexiconHasSynonyms(t *testing.T) {
	lex := New()
	lex.AddSynonymGroup("analyze", []string{"analyze", "analysis", "analytical"})

	tests := []struct {
		token string
		want  bool
	}{
		{"analyze", true},
		{"analysis", true},
		{"analytical", true},
		{"ANALYSIS", true}, // Case insensitive
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := lex.HasSynonyms(tt.token); got != tt.want {
			t.Errorf("HasSynonyms(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

func TestLexiconCTokens(t *testing.T) {
	lex := New()

	// Add c-token relationships
	lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.5, Support: 100})
	lex.AddCToken("transformer", CToken{Token: "bert", PMI: 3.2, Support: 80})
	lex.AddCToken("transformer", CToken{Token: "gpt", PMI: 2.8, Support: 90})

	// Retrieve c-tokens
	ctokens := lex.GetCTokens("transformer")
	if len(ctokens) != 3 {
		t.Fatalf("GetCTokens('transformer') returned %d tokens, want 3", len(ctokens))
	}

	// Verify specific c-token
	found := false
	for _, ct := range ctokens {
		if ct.Token == "attention" && ct.PMI == 2.5 && ct.Support == 100 {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetCTokens('transformer') missing expected c-token 'attention'")
	}

	// Unknown token should return empty
	unknown := lex.GetCTokens("unknown")
	if len(unknown) != 0 {
		t.Errorf("GetCTokens('unknown') returned %d tokens, want 0", len(unknown))
	}
}

func TestLexiconStats(t *testing.T) {
	lex := New()

	// Add synonym groups
	lex.AddSynonymGroup("game", []string{"game", "games", "gaming"})
	lex.AddSynonymGroup("analyze", []string{"analyze", "analysis"})

	// Add c-tokens
	lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.5, Support: 100})
	lex.AddCToken("transformer", CToken{Token: "bert", PMI: 3.2, Support: 80})
	lex.AddCToken("neural", CToken{Token: "network", PMI: 1.8, Support: 200})

	stats := lex.Stats()

	if stats.SynonymGroups != 2 {
		t.Errorf("Stats.SynonymGroups = %d, want 2", stats.SynonymGroups)
	}
	if stats.TotalVariants != 5 {  // game, games, gaming, analyze, analysis
		t.Errorf("Stats.TotalVariants = %d, want 5", stats.TotalVariants)
	}
	if stats.CTokenEntries != 2 {  // transformer, neural
		t.Errorf("Stats.CTokenEntries = %d, want 2", stats.CTokenEntries)
	}
	if stats.TotalCTokens != 3 {
		t.Errorf("Stats.TotalCTokens = %d, want 3", stats.TotalCTokens)
	}
}

func TestLexiconLoadFromYAML(t *testing.T) {
	// Create temporary YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "synonyms.yaml")

	yamlContent := `synonyms:
  - canonical: game
    variants: [games, gaming, gamer]
  - canonical: ml
    variants: [machine learning, machine-learning, ML]
  - canonical: analyze
    variants: [analysis, analytical, analyzer]
`

	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Load lexicon
	lex, err := LoadFromYAML(yamlPath)
	if err != nil {
		t.Fatalf("LoadFromYAML failed: %v", err)
	}

	// Test loaded data
	tests := []struct {
		token string
		want  string
	}{
		{"gaming", "game"},
		{"gamer", "game"},
		{"machine learning", "ml"},
		{"ML", "ml"},
		{"analysis", "analyze"},
		{"analytical", "analyze"},
	}

	for _, tt := range tests {
		if got := lex.Normalize(tt.token); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.token, got, tt.want)
		}
	}

	// Test stats
	stats := lex.Stats()
	if stats.SynonymGroups != 3 {
		t.Errorf("Loaded lexicon has %d synonym groups, want 3", stats.SynonymGroups)
	}
}

func TestLexiconLoadFromYAMLInvalidFile(t *testing.T) {
	// Test loading non-existent file
	_, err := LoadFromYAML("/nonexistent/path.yaml")
	if err == nil {
		t.Error("LoadFromYAML with nonexistent file should return error")
	}

	// Test loading invalid YAML
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	_, err = LoadFromYAML(invalidPath)
	if err == nil {
		t.Error("LoadFromYAML with invalid YAML should return error")
	}
}

func TestLexiconMultiTokenVariants(t *testing.T) {
	lex := New()

	// Add multi-token variants
	lex.AddSynonymGroup("anova", []string{"anova", "analysis of variance", "analysis~variance"})

	// Test normalization of multi-token phrases
	if got := lex.Normalize("analysis of variance"); got != "anova" {
		t.Errorf("Normalize('analysis of variance') = %q, want 'anova'", got)
	}

	// Test variants expansion
	variants := lex.Variants("anova")
	found := false
	for _, v := range variants {
		if v == "analysis of variance" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Variants('anova') missing multi-token variant 'analysis of variance'")
	}
}

func TestLexiconDuplicateVariants(t *testing.T) {
	lex := New()

	// Add synonym group with duplicates
	lex.AddSynonymGroup("test", []string{"test", "test", "testing", "testing"})

	// Should de-duplicate
	variants := lex.Variants("test")
	if len(variants) != 2 { // test, testing
		t.Errorf("Variants should be deduplicated, got %d variants: %v", len(variants), variants)
	}
}

func TestLexiconCTokensCaseInsensitive(t *testing.T) {
	lex := New()

	// Add with different cases
	lex.AddCToken("Transformer", CToken{Token: "Attention", PMI: 2.5, Support: 100})

	// Retrieve with different case
	ctokens := lex.GetCTokens("TRANSFORMER")
	if len(ctokens) != 1 {
		t.Fatalf("GetCTokens should be case-insensitive, got %d tokens", len(ctokens))
	}
	if ctokens[0].Token != "attention" {
		t.Errorf("C-token should be normalized to lowercase, got %q", ctokens[0].Token)
	}
}

func TestLexiconVariantsFromCanonical(t *testing.T) {
	lex := New()
	lex.AddSynonymGroup("analyze", []string{"analyze", "analysis", "analytical"})

	// Getting variants from canonical form
	variants := lex.Variants("analyze")
	if len(variants) != 3 {
		t.Errorf("Variants('analyze') = %d items, want 3", len(variants))
	}

	// Getting variants from non-canonical form
	variantsFromAlias := lex.Variants("analysis")
	if len(variantsFromAlias) != 3 {
		t.Errorf("Variants('analysis') = %d items, want 3", len(variantsFromAlias))
	}

	// Both should return the same set
	if len(variants) != len(variantsFromAlias) {
		t.Error("Variants should return same set regardless of input form")
	}
}

func TestLexiconReverseIndexCleanup(t *testing.T) {
	lex := New()

	// Add initial synonym group
	lex.AddSynonymGroup("game", []string{"games", "gaming"})

	// Verify initial state
	if lex.Normalize("gaming") != "game" {
		t.Error("Initial: gaming should normalize to game")
	}

	// Re-add with different variants (dropping "gaming", adding "gamer")
	lex.AddSynonymGroup("game", []string{"games", "gamer"})

	// Old variant "gaming" should no longer normalize to "game"
	if lex.Normalize("gaming") != "gaming" {
		t.Error("After re-add: gaming should normalize to itself (dropped from group)")
	}

	// New variant "gamer" should normalize to "game"
	if lex.Normalize("gamer") != "game" {
		t.Error("After re-add: gamer should normalize to game")
	}

	// Existing variant "games" should still work
	if lex.Normalize("games") != "game" {
		t.Error("After re-add: games should still normalize to game")
	}
}

func TestLexiconCanonicalAlwaysIncluded(t *testing.T) {
	lex := New()

	// Add group without including canonical in variants list
	lex.AddSynonymGroup("analyze", []string{"analysis", "analytical"})

	// Canonical should be included automatically
	variants := lex.Variants("analyze")
	found := false
	for _, v := range variants {
		if v == "analyze" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Canonical 'analyze' should be included in variants even if not in input list")
	}

	// Canonical should normalize to itself
	if lex.Normalize("analyze") != "analyze" {
		t.Error("Canonical should normalize to itself")
	}

	// Canonical should be first
	if len(variants) > 0 && variants[0] != "analyze" {
		t.Errorf("Canonical should be first variant, got %q", variants[0])
	}
}

func TestLexiconCTokenDeduplication(t *testing.T) {
	lex := New()

	// Add same c-token multiple times with different PMI
	lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.0, Support: 50})
	lex.AddCToken("transformer", CToken{Token: "attention", PMI: 2.5, Support: 60})  // Higher PMI
	lex.AddCToken("transformer", CToken{Token: "attention", PMI: 1.8, Support: 100}) // Lower PMI, higher support

	ctokens := lex.GetCTokens("transformer")

	// Should only have one entry for "attention"
	attentionCount := 0
	var finalCToken CToken
	for _, ct := range ctokens {
		if ct.Token == "attention" {
			attentionCount++
			finalCToken = ct
		}
	}

	if attentionCount != 1 {
		t.Errorf("Should have exactly 1 c-token for 'attention', got %d", attentionCount)
	}

	// Should keep the one with highest PMI (2.5)
	if finalCToken.PMI != 2.5 {
		t.Errorf("Should keep highest PMI, got PMI=%.1f, want 2.5", finalCToken.PMI)
	}
	if finalCToken.Support != 60 {
		t.Errorf("Should have support from highest PMI entry, got %d, want 60", finalCToken.Support)
	}
}

func TestLexiconCTokenEqualPMI(t *testing.T) {
	lex := New()

	// Add c-tokens with equal PMI but different support
	lex.AddCToken("test", CToken{Token: "related", PMI: 2.0, Support: 50})
	lex.AddCToken("test", CToken{Token: "related", PMI: 2.0, Support: 100}) // Same PMI, higher support

	ctokens := lex.GetCTokens("test")
	if len(ctokens) != 1 {
		t.Fatalf("Should have 1 c-token, got %d", len(ctokens))
	}

	// Should keep the one with higher support when PMI is equal
	if ctokens[0].Support != 100 {
		t.Errorf("Should keep higher support when PMI equal, got %d, want 100", ctokens[0].Support)
	}
}
