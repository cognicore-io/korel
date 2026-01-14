package pmi

import (
	"testing"
)

func TestCounterBasic(t *testing.T) {
	counter := NewCounter()

	doc1 := []string{"machine-learning", "python", "data"}
	counter.AddDocument(doc1)

	if counter.TotalDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", counter.TotalDocs())
	}

	if counter.GetTokenCount("python") != 1 {
		t.Error("Token 'python' should have count 1")
	}
}

func TestCounterCooccurrence(t *testing.T) {
	counter := NewCounter()

	doc1 := []string{"machine-learning", "python"}
	doc2 := []string{"machine-learning", "python"}
	counter.AddDocument(doc1)
	counter.AddDocument(doc2)

	// Both appear in 2 docs
	if counter.GetTokenCount("machine-learning") != 2 {
		t.Error("machine-learning should appear in 2 docs")
	}

	// Co-occur in 2 docs
	count := counter.GetPairCount("machine-learning", "python")
	if count != 2 {
		t.Errorf("Pair should co-occur 2 times, got %d", count)
	}
}

func TestCounterCanonicalOrdering(t *testing.T) {
	counter := NewCounter()

	doc := []string{"zebra", "apple"}
	counter.AddDocument(doc)

	// Should work regardless of order
	count1 := counter.GetPairCount("zebra", "apple")
	count2 := counter.GetPairCount("apple", "zebra")

	if count1 != count2 {
		t.Error("Pair count should be symmetric")
	}

	if count1 != 1 {
		t.Errorf("Expected count 1, got %d", count1)
	}
}

func TestCounterMultipleDocuments(t *testing.T) {
	counter := NewCounter()

	docs := [][]string{
		{"a", "b"},
		{"a", "c"},
		{"b", "c"},
		{"a", "b", "c"},
	}

	for _, doc := range docs {
		counter.AddDocument(doc)
	}

	if counter.TotalDocs() != 4 {
		t.Errorf("Expected 4 docs, got %d", counter.TotalDocs())
	}

	// "a" appears in 3 docs
	if counter.GetTokenCount("a") != 3 {
		t.Errorf("Token 'a' should appear in 3 docs, got %d", counter.GetTokenCount("a"))
	}

	// "a" and "b" co-occur in 2 docs
	if counter.GetPairCount("a", "b") != 2 {
		t.Errorf("Pair (a,b) should co-occur 2 times, got %d", counter.GetPairCount("a", "b"))
	}

	// "a" and "c" co-occur in 2 docs
	if counter.GetPairCount("a", "c") != 2 {
		t.Error("Pair (a,c) should co-occur 2 times")
	}

	// "b" and "c" co-occur in 2 docs
	if counter.GetPairCount("b", "c") != 2 {
		t.Error("Pair (b,c) should co-occur 2 times")
	}
}

func TestCounterUniqueTokensPerDoc(t *testing.T) {
	counter := NewCounter()

	// AddDocument expects unique tokens per doc (caller's responsibility)
	// If duplicates are passed, they will be counted
	doc := []string{"python"}
	counter.AddDocument(doc)

	if counter.GetTokenCount("python") != 1 {
		t.Error("Token should appear in 1 document")
	}

	// Add same token in another doc
	counter.AddDocument([]string{"python"})
	if counter.GetTokenCount("python") != 2 {
		t.Error("Token should now appear in 2 documents")
	}
}

func TestCounterUniquePairs(t *testing.T) {
	counter := NewCounter()

	doc1 := []string{"a", "b", "c"}
	counter.AddDocument(doc1)

	// 3 tokens → 3 pairs: (a,b), (a,c), (b,c)
	if counter.UniquePairs() != 3 {
		t.Errorf("Expected 3 unique pairs, got %d", counter.UniquePairs())
	}
}

func TestCounterUniqueTokens(t *testing.T) {
	counter := NewCounter()

	counter.AddDocument([]string{"a", "b"})
	counter.AddDocument([]string{"b", "c"})

	// Unique tokens: a, b, c
	if counter.UniqueTokens() != 3 {
		t.Errorf("Expected 3 unique tokens, got %d", counter.UniqueTokens())
	}
}

func TestCounterEmptyDocument(t *testing.T) {
	counter := NewCounter()

	counter.AddDocument([]string{})

	if counter.TotalDocs() != 1 {
		t.Error("Empty document should still increment doc count")
	}

	if counter.UniqueTokens() != 0 {
		t.Error("Empty document should not add tokens")
	}
}

func TestCounterNonExistentToken(t *testing.T) {
	counter := NewCounter()

	counter.AddDocument([]string{"a", "b"})

	if counter.GetTokenCount("nonexistent") != 0 {
		t.Error("Non-existent token should have count 0")
	}

	if counter.GetPairCount("nonexistent", "also-nonexistent") != 0 {
		t.Error("Non-existent pair should have count 0")
	}
}
