package ingest

import (
	"testing"
	"time"
)

func TestDocCreation(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Test Article",
		Outlet:      "Tech News",
		PublishedAt: time.Now(),
		BodyText:    "This is a test article.",
		LinksOut:    []string{"https://link1.com", "https://link2.com"},
		SourceCats:  []string{"technology", "ai"},
	}

	if doc.URL == "" {
		t.Error("URL should be set")
	}

	if doc.Title == "" {
		t.Error("Title should be set")
	}

	if len(doc.LinksOut) != 2 {
		t.Errorf("Expected 2 links, got %d", len(doc.LinksOut))
	}

	if len(doc.SourceCats) != 2 {
		t.Errorf("Expected 2 source categories, got %d", len(doc.SourceCats))
	}
}

func TestDocValidate(t *testing.T) {
	// Valid doc
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Test Article",
		PublishedAt: time.Now(),
		BodyText:    "Some content here",
	}

	err := doc.Validate()
	if err != nil {
		t.Errorf("Valid doc should pass validation, got %v", err)
	}
}

func TestDocValidateMissingURL(t *testing.T) {
	doc := Doc{
		URL:         "",
		Title:       "Test Article",
		PublishedAt: time.Now(),
		BodyText:    "Content",
	}

	err := doc.Validate()
	if err == nil {
		t.Error("Should fail validation without URL")
	}
}

func TestDocValidateMissingTitle(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "",
		PublishedAt: time.Now(),
		BodyText:    "Content",
	}

	err := doc.Validate()
	if err == nil {
		t.Error("Should fail validation without title")
	}
}

func TestDocValidateMissingPublishedAt(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Test",
		PublishedAt: time.Time{}, // zero time
		BodyText:    "Content",
	}

	err := doc.Validate()
	if err == nil {
		t.Error("Should fail validation without published time")
	}
}

func TestDocValidateMissingBodyText(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Test",
		PublishedAt: time.Now(),
		BodyText:    "",
	}

	err := doc.Validate()
	if err == nil {
		t.Error("Should fail validation without body text")
	}
}

func TestDocValidateWhitespaceOnly(t *testing.T) {
	doc := Doc{
		URL:         "   ",
		Title:       "\t\n",
		PublishedAt: time.Now(),
		BodyText:    "  ",
	}

	err := doc.Validate()
	if err == nil {
		t.Error("Should fail validation with whitespace-only fields")
	}
}

func TestDocEmptyFields(t *testing.T) {
	// Test doc with empty fields
	doc := Doc{}

	if doc.URL != "" {
		t.Error("Empty doc should have empty URL")
	}

	if len(doc.LinksOut) != 0 {
		t.Error("Empty doc should have no links")
	}

	if len(doc.SourceCats) != 0 {
		t.Error("Empty doc should have no source categories")
	}
}

func TestDocWithNoLinks(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Article with no links",
		PublishedAt: time.Now(),
		LinksOut:    []string{},
	}

	if len(doc.LinksOut) != 0 {
		t.Error("Should have no links")
	}
}

func TestDocWithNoSourceCategories(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		Title:       "Article with no categories",
		PublishedAt: time.Now(),
		SourceCats:  []string{},
	}

	if len(doc.SourceCats) != 0 {
		t.Error("Should have no source categories")
	}
}

func TestDocWithManyLinks(t *testing.T) {
	// Create doc with many links
	links := make([]string, 100)
	for i := 0; i < 100; i++ {
		links[i] = "https://example.com/link" + string(rune(i))
	}

	doc := Doc{
		URL:      "https://example.com/article",
		LinksOut: links,
	}

	if len(doc.LinksOut) != 100 {
		t.Errorf("Expected 100 links, got %d", len(doc.LinksOut))
	}
}

func TestDocWithLongBody(t *testing.T) {
	// Create doc with very long body
	longBody := ""
	for i := 0; i < 10000; i++ {
		longBody += "word "
	}

	doc := Doc{
		URL:      "https://example.com/article",
		BodyText: longBody,
	}

	if len(doc.BodyText) == 0 {
		t.Error("Body text should not be empty")
	}
}

func TestDocZeroTime(t *testing.T) {
	doc := Doc{
		URL:         "https://example.com/article",
		PublishedAt: time.Time{}, // zero time
	}

	if !doc.PublishedAt.IsZero() {
		t.Error("PublishedAt should be zero time")
	}
}

func TestDocFutureTime(t *testing.T) {
	future := time.Now().Add(365 * 24 * time.Hour) // 1 year in future

	doc := Doc{
		URL:         "https://example.com/article",
		PublishedAt: future,
	}

	if doc.PublishedAt.Before(time.Now()) {
		t.Error("PublishedAt should be in the future")
	}
}

func TestDocVeryOldTime(t *testing.T) {
	old := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)

	doc := Doc{
		URL:         "https://example.com/article",
		PublishedAt: old,
	}

	if doc.PublishedAt.After(time.Now()) {
		t.Error("PublishedAt should be in the past")
	}
}

func TestDocDuplicateLinks(t *testing.T) {
	// Doc with duplicate links (not filtered, just stored)
	doc := Doc{
		URL: "https://example.com/article",
		LinksOut: []string{
			"https://link.com",
			"https://link.com",
			"https://link.com",
		},
	}

	// Doc struct doesn't deduplicate
	if len(doc.LinksOut) != 3 {
		t.Errorf("Should preserve duplicate links, got %d", len(doc.LinksOut))
	}
}

func TestDocDuplicateCategories(t *testing.T) {
	// Doc with duplicate categories (not filtered, just stored)
	doc := Doc{
		URL: "https://example.com/article",
		SourceCats: []string{
			"tech",
			"tech",
			"ai",
			"tech",
		},
	}

	// Doc struct doesn't deduplicate
	if len(doc.SourceCats) != 4 {
		t.Errorf("Should preserve duplicate categories, got %d", len(doc.SourceCats))
	}
}

func TestDocEmptyBodyText(t *testing.T) {
	doc := Doc{
		URL:      "https://example.com/article",
		Title:    "Article with no body",
		BodyText: "",
	}

	if doc.BodyText != "" {
		t.Error("BodyText should be empty")
	}
}

func TestDocWhitespaceOnlyBody(t *testing.T) {
	doc := Doc{
		URL:      "https://example.com/article",
		BodyText: "   \t\n\r   ",
	}

	// Doc doesn't trim whitespace, just stores it
	if len(doc.BodyText) == 0 {
		t.Error("Should preserve whitespace in BodyText")
	}
}

func TestDocSpecialCharactersInURL(t *testing.T) {
	doc := Doc{
		URL: "https://example.com/article?id=123&query=test%20value",
	}

	if doc.URL == "" {
		t.Error("URL with special characters should be stored")
	}
}

func TestDocUnicodeInTitle(t *testing.T) {
	doc := Doc{
		Title: "Café résumé naïve 日本語 中文",
	}

	if doc.Title == "" {
		t.Error("Title with unicode should be stored")
	}
}

func TestDocEmptyOutlet(t *testing.T) {
	doc := Doc{
		URL:    "https://example.com/article",
		Outlet: "",
	}

	if doc.Outlet != "" {
		t.Error("Outlet should be empty")
	}
}
