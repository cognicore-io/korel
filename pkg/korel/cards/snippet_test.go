package cards

import (
	"strings"
	"testing"
)

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Hello world.", 1},
		{"First sentence. Second sentence.", 2},
		{"One. Two. Three. Four.", 4},
		{"No period at end", 1},
		{"Question? Yes! And a statement.", 3},
		{"", 0},
		{"U.S. policy is complex. It really is.", 3}, // simple splitter treats U.S. as boundary
	}
	for _, tt := range tests {
		got := splitSentences(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitSentences(%q) = %d sentences %v, want %d", tt.input, len(got), got, tt.want)
		}
	}
}

func TestExtractSnippet(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := ExtractSnippet(""); got != "" {
			t.Errorf("ExtractSnippet empty = %q, want empty", got)
		}
	})

	t.Run("short_text", func(t *testing.T) {
		got := ExtractSnippet("Short text.")
		if got != "Short text." {
			t.Errorf("got %q", got)
		}
	})

	t.Run("three_sentences", func(t *testing.T) {
		text := "First. Second. Third. Fourth. Fifth."
		got := ExtractSnippet(text)
		sentences := splitSentences(got)
		if len(sentences) > 3 {
			t.Errorf("got %d sentences, want <= 3: %q", len(sentences), got)
		}
	})

	t.Run("long_text_truncated", func(t *testing.T) {
		long := strings.Repeat("Word ", 200) + "end."
		got := ExtractSnippet(long)
		if len([]rune(got)) > maxSnippetLen+10 { // +10 for "..."
			t.Errorf("snippet too long: %d runes", len([]rune(got)))
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("truncated snippet should end with ..., got %q", got[len(got)-20:])
		}
	})
}

func TestGenerateBullet(t *testing.T) {
	queryTokens := map[string]struct{}{
		"machine-learning": {},
		"neural-network":   {},
	}

	t.Run("no_snippet_uses_title", func(t *testing.T) {
		doc := ScoredDoc{Title: "My Title", BodySnippet: ""}
		got := generateBullet(doc, queryTokens)
		if got != "My Title" {
			t.Errorf("got %q, want title", got)
		}
	})

	t.Run("prefers_sentence_with_query_tokens", func(t *testing.T) {
		doc := ScoredDoc{
			Title:       "Paper Title",
			BodySnippet: "Paper Title. This covers neural network architectures. Another topic here.",
		}
		got := generateBullet(doc, queryTokens)
		if !strings.Contains(strings.ToLower(got), "neural network") {
			t.Errorf("expected sentence with 'neural network', got %q", got)
		}
	})

	t.Run("skips_title_duplicate", func(t *testing.T) {
		doc := ScoredDoc{
			Title:       "Breaking News",
			BodySnippet: "Breaking News. The actual content is here.",
		}
		got := generateBullet(doc, map[string]struct{}{})
		if got == "Breaking News" {
			t.Errorf("should skip title-duplicate bullet, got %q", got)
		}
	})
}
