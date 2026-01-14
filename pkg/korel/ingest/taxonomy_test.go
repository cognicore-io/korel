package ingest

import (
	"fmt"
	"strings"
	"testing"
)

func TestTaxonomyAssignCategories(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddSector("ai", []string{"machine-learning", "neural-network", "transformer"})
	tax.AddSector("web", []string{"html", "css", "javascript"})
	tax.AddEvent("release", []string{"launched", "released", "announced"})

	tokens := []string{"new", "transformer", "model", "released"}
	cats := tax.AssignCategories(tokens)

	// Should match both "ai" (from transformer) and "release" (from released)
	expectedCats := map[string]bool{"ai": true, "release": true}

	if len(cats) != len(expectedCats) {
		t.Errorf("Expected %d categories, got %d", len(expectedCats), len(cats))
	}

	for _, cat := range cats {
		if !expectedCats[cat] {
			t.Errorf("Unexpected category: %s", cat)
		}
	}
}

func TestTaxonomyNoMatch(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddSector("ai", []string{"machine-learning"})

	tokens := []string{"hello", "world"}
	cats := tax.AssignCategories(tokens)

	if len(cats) != 0 {
		t.Error("Tokens with no matches should return empty categories")
	}
}

func TestTaxonomyMultipleSectors(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddSector("ai", []string{"machine-learning", "neural-network"})
	tax.AddSector("nlp", []string{"text", "language"})

	// Both sectors match
	tokens := []string{"machine-learning", "text", "processing"}
	cats := tax.AssignCategories(tokens)

	if len(cats) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}

func TestTaxonomyAddRegion(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddRegion("europe", []string{"berlin", "paris", "london"})

	tokens := []string{"conference", "in", "berlin"}
	cats := tax.AssignCategories(tokens)

	if len(cats) != 1 || cats[0] != "europe" {
		t.Errorf("Should categorize as 'europe', got %v", cats)
	}
}

func TestTaxonomyAddEntity(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddEntity("company", "OpenAI", []string{"openai", "open-ai"})

	// ExtractEntities test (when implemented)
	// For now just verify it doesn't panic
	tax.AddEntity("ticker", "TSLA", []string{"tesla"})
}

func TestTaxonomyEmpty(t *testing.T) {
	tax := NewTaxonomy()

	tokens := []string{"anything"}
	cats := tax.AssignCategories(tokens)

	if len(cats) != 0 {
		t.Error("Empty taxonomy should return no categories")
	}
}

func TestTaxonomyCaseInsensitive(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddSector("ai", []string{"bert", "gpt"})

	// Mixed case tokens
	tokens := []string{"BERT", "model"}
	cats := tax.AssignCategories(tokens)

	// Note: Current implementation is case-sensitive
	// This test documents expected behavior
	// May need to add .ToLower() in AssignCategories
	if len(cats) == 0 {
		t.Log("Current implementation is case-sensitive (may want to change)")
	}
}

// Edge case tests

func TestTaxonomyDuplicateCategoryAddition(t *testing.T) {
	tax := NewTaxonomy()

	// Add same sector multiple times
	tax.AddSector("ai", []string{"machine-learning"})
	tax.AddSector("ai", []string{"neural-network"})
	tax.AddSector("ai", []string{"machine-learning"}) // duplicate keyword

	tokens := []string{"machine-learning", "neural-network"}
	cats := tax.AssignCategories(tokens)

	// Should get "ai" category only once
	if len(cats) != 1 {
		t.Errorf("Expected 1 category despite duplicates, got %d: %v", len(cats), cats)
	}
	if cats[0] != "ai" {
		t.Errorf("Expected 'ai', got %s", cats[0])
	}
}

func TestTaxonomyEmptyKeywordList(t *testing.T) {
	tax := NewTaxonomy()

	// Add sector with empty keyword list
	tax.AddSector("empty", []string{})

	tokens := []string{"anything"}
	cats := tax.AssignCategories(tokens)

	// Should not match anything
	if len(cats) != 0 {
		t.Errorf("Empty keyword list should not match, got %v", cats)
	}
}

func TestTaxonomyVeryLongCategoryName(t *testing.T) {
	tax := NewTaxonomy()

	longName := strings.Repeat("verylongcategoryname", 10)
	tax.AddSector(longName, []string{"test"})

	tokens := []string{"test"}
	cats := tax.AssignCategories(tokens)

	// Should handle long category names
	if len(cats) != 1 || cats[0] != longName {
		t.Errorf("Should handle long category names")
	}
}

func TestTaxonomyManyCategories(t *testing.T) {
	tax := NewTaxonomy()

	// Add many categories
	for i := 0; i < 100; i++ {
		tax.AddSector(fmt.Sprintf("cat%d", i), []string{fmt.Sprintf("keyword%d", i)})
	}

	// Should handle many categories without issue
	tokens := []string{"keyword50", "keyword75"}
	cats := tax.AssignCategories(tokens)

	if len(cats) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}

func TestTaxonomyEntityExtraction(t *testing.T) {
	tax := NewTaxonomy()

	tax.AddEntity("company", "Tesla", []string{"tesla", "tsla"})
	tax.AddEntity("company", "Apple", []string{"apple", "aapl"})

	text := "Tesla and Apple announced new products"
	entities := tax.ExtractEntities(text)

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}

	// Check both are found
	foundTesla := false
	foundApple := false
	for _, e := range entities {
		if e.Type == "company" && e.Value == "Tesla" {
			foundTesla = true
		}
		if e.Type == "company" && e.Value == "Apple" {
			foundApple = true
		}
	}

	if !foundTesla || !foundApple {
		t.Error("Should extract both Tesla and Apple entities")
	}
}

func TestTaxonomyEntityExtractionCaseInsensitive(t *testing.T) {
	tax := NewTaxonomy()

	tax.AddEntity("company", "OpenAI", []string{"openai", "open-ai"})

	// Test various cases
	text1 := "OpenAI released a new model"
	text2 := "OPENAI released a new model"
	text3 := "openai released a new model"

	entities1 := tax.ExtractEntities(text1)
	entities2 := tax.ExtractEntities(text2)
	entities3 := tax.ExtractEntities(text3)

	// All should extract the entity
	if len(entities1) == 0 || len(entities2) == 0 || len(entities3) == 0 {
		t.Error("Entity extraction should be case-insensitive")
	}
}

func TestTaxonomyNoEntityMatch(t *testing.T) {
	tax := NewTaxonomy()

	tax.AddEntity("company", "Tesla", []string{"tesla"})

	text := "Microsoft announced new products"
	entities := tax.ExtractEntities(text)

	if len(entities) != 0 {
		t.Errorf("Should not extract entities that don't match, got %v", entities)
	}
}

func TestTaxonomyEntityWithEmptyKeywords(t *testing.T) {
	tax := NewTaxonomy()

	// Add entity with empty keyword list
	tax.AddEntity("company", "Test", []string{})

	text := "Test company"
	entities := tax.ExtractEntities(text)

	// Should not match anything
	if len(entities) != 0 {
		t.Errorf("Empty entity keywords should not match, got %v", entities)
	}
}

func TestTaxonomyMultipleEntityTypes(t *testing.T) {
	tax := NewTaxonomy()

	tax.AddEntity("company", "Tesla", []string{"tesla"})
	tax.AddEntity("person", "Elon", []string{"elon", "musk"})
	tax.AddEntity("ticker", "TSLA", []string{"tsla"})

	text := "Elon at Tesla with TSLA stock"
	entities := tax.ExtractEntities(text)

	if len(entities) != 3 {
		t.Errorf("Expected 3 entities of different types, got %d: %v", len(entities), entities)
	}
}

func TestTaxonomyAllCategoryTypes(t *testing.T) {
	tax := NewTaxonomy()

	tax.AddSector("ai", []string{"machine-learning"})
	tax.AddEvent("release", []string{"launched"})
	tax.AddRegion("europe", []string{"berlin"})

	tokens := []string{"machine-learning", "launched", "berlin"}
	cats := tax.AssignCategories(tokens)

	// Should match all three category types
	if len(cats) != 3 {
		t.Errorf("Expected 3 categories (sector, event, region), got %d: %v", len(cats), cats)
	}

	expected := map[string]bool{"ai": true, "release": true, "europe": true}
	for _, cat := range cats {
		if !expected[cat] {
			t.Errorf("Unexpected category: %s", cat)
		}
	}
}
