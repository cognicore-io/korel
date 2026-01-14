package main

import (
	"strings"
	"testing"
)

// TestStripHTML tests HTML tag removal
func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple paragraph",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "multiple tags",
			input: "<div><p>Hello</p><p>World</p></div>",
			want:  "HelloWorld",
		},
		{
			name:  "with attributes",
			input: `<a href="https://example.com">Link text</a>`,
			want:  "Link text",
		},
		{
			name:  "nested tags",
			input: "<p><strong>Bold</strong> and <em>italic</em></p>",
			want:  "Bold and italic",
		},
		{
			name:  "plain text",
			input: "No HTML here",
			want:  "No HTML here",
		},
		{
			name:  "with newlines",
			input: "<p>Line 1</p>\n<p>Line 2</p>",
			want:  "Line 1\nLine 2",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   \t\n  ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestCategorize tests keyword-based categorization
func TestCategorize(t *testing.T) {
	tests := []struct {
		name  string
		title string
		url   string
		want  []string
	}{
		{
			name:  "AI keywords",
			title: "New GPT-4 Model Released",
			url:   "https://openai.com/blog",
			want:  []string{"ai"},
		},
		{
			name:  "startup keywords",
			title: "Startup Raises $10M Series A",
			url:   "",
			want:  []string{"ai", "startup"}, // "raises" contains "ai"
		},
		{
			name:  "programming keywords",
			title: "New Python Framework for Web Development",
			url:   "",
			want:  []string{"programming", "web"}, // matches both framework and web
		},
		{
			name:  "security keywords",
			title: "Critical Security Vulnerability in OpenSSL",
			url:   "",
			want:  []string{"security"},
		},
		{
			name:  "web keywords",
			title: "Chrome 120 Released with New Features",
			url:   "",
			want:  []string{"web"},
		},
		{
			name:  "open source keywords",
			title: "GitHub Announces New Open Source Initiative",
			url:   "",
			want:  []string{"opensource"},
		},
		{
			name:  "multiple categories",
			title: "AI Startup Raises Funding for Open Source Framework",
			url:   "",
			want:  []string{"ai", "startup", "programming", "opensource"}, // framework → programming
		},
		{
			name:  "no keywords - defaults to tech",
			title: "Random Tech News",
			url:   "",
			want:  []string{"tech"},
		},
		{
			name:  "case insensitive matching",
			title: "MACHINE LEARNING and NEURAL networks",
			url:   "",
			want:  []string{"ai"},
		},
		{
			name:  "keywords in URL",
			title: "Interesting Article",
			url:   "https://github.com/repo/ai-framework",
			want:  []string{"ai", "programming", "opensource"}, // github → opensource, framework → programming
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorize(tt.title, tt.url)
			if !equalStringSlices(got, tt.want) {
				t.Errorf("categorize(%q, %q) = %v, want %v", tt.title, tt.url, got, tt.want)
			}
		})
	}
}

// TestContainsAny tests the helper function
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		keywords []string
		want     bool
	}{
		{
			name:     "single match",
			s:        "machine learning is awesome",
			keywords: []string{"machine learning"},
			want:     true,
		},
		{
			name:     "no match",
			s:        "random text here",
			keywords: []string{"ai", "gpt"},
			want:     false,
		},
		{
			name:     "multiple keywords - one matches",
			s:        "neural networks are cool",
			keywords: []string{"ai", "neural", "gpt"},
			want:     true,
		},
		{
			name:     "empty string",
			s:        "",
			keywords: []string{"test"},
			want:     false,
		},
		{
			name:     "empty keywords",
			s:        "some text",
			keywords: []string{},
			want:     false,
		},
		{
			name:     "partial match",
			s:        "programming language",
			keywords: []string{"program"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.s, tt.keywords...)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.keywords, got, tt.want)
			}
		})
	}
}

// TestCategorizationEdgeCases tests edge cases in categorization
func TestCategorizationEdgeCases(t *testing.T) {
	// Test empty inputs
	cats := categorize("", "")
	if len(cats) == 0 || cats[0] != "tech" {
		t.Errorf("Empty title/URL should default to 'tech', got %v", cats)
	}

	// Test very long title
	longTitle := strings.Repeat("word ", 1000)
	cats = categorize(longTitle, "")
	if len(cats) == 0 {
		t.Error("Very long title should still be categorized")
	}

	// Test special characters
	cats = categorize("$%^&*() AI @#$%", "")
	if !contains(cats, "ai") {
		t.Errorf("Should find 'ai' despite special characters, got %v", cats)
	}

	// Test all categories at once
	cats = categorize("AI startup programming security web open source", "")
	if len(cats) < 5 {
		t.Errorf("Should match multiple categories, got %v", cats)
	}
}

// Helper functions

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int)
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
