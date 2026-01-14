package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// arXiv API endpoint
const apiURL = "http://export.arxiv.org/api/query"

// ArxivFeed represents the XML response from arXiv API
type ArxivFeed struct {
	XMLName xml.Name      `xml:"feed"`
	Entries []ArxivEntry  `xml:"entry"`
}

// ArxivEntry represents a single paper
type ArxivEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Summary   string   `xml:"summary"`
	Published string   `xml:"published"`
	Authors   []Author `xml:"author"`
	Category  []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
	Link []struct {
		Href string `xml:"href,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
}

type Author struct {
	Name string `xml:"name"`
}

// KorelDoc is the format for Korel ingestion
type KorelDoc struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
	Outlet      string    `json:"outlet"`
	Text        string    `json:"text"`
	SourceCats  []string  `json:"source_cats"`
}

func main() {
	// Configuration
	category := "cs.AI"  // Default: AI papers
	maxResults := 200

	if len(os.Args) > 1 {
		category = os.Args[1]
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &maxResults)
	}

	log.Printf("Downloading %d papers from arXiv category: %s\n", maxResults, category)
	log.Println("Categories: cs.AI (AI), cs.CL (NLP), cs.LG (ML), econ.EM (Economics), q-fin (Finance)")

	// Build query
	params := url.Values{}
	params.Set("search_query", "cat:"+category)
	params.Set("max_results", fmt.Sprintf("%d", maxResults))
	params.Set("sortBy", "submittedDate")
	params.Set("sortOrder", "descending")

	fullURL := apiURL + "?" + params.Encode()

	// Fetch from arXiv
	log.Println("Fetching from arXiv API...")
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Fatal("Failed to fetch:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("HTTP error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response:", err)
	}

	// Parse XML
	var feed ArxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		log.Fatal("Failed to parse XML:", err)
	}

	log.Printf("Received %d papers\n", len(feed.Entries))

	// Create output directory
	outDir := "testdata/arxiv"
	os.MkdirAll(outDir, 0755)

	// Open output file
	outFile, err := os.Create(outDir + "/docs.jsonl")
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	downloaded := 0

	// Process each entry
	for _, entry := range feed.Entries {
		// Parse publication date
		pubTime, err := time.Parse("2006-01-02T15:04:05Z", entry.Published)
		if err != nil {
			pubTime = time.Now()
		}

		// Extract categories
		cats := []string{}
		for _, cat := range entry.Category {
			cats = append(cats, mapArxivCategory(cat.Term))
		}
		if len(cats) == 0 {
			cats = []string{"research"}
		}

		// Build text body
		text := cleanText(entry.Title) + ". " + cleanText(entry.Summary)

		// Extract authors
		authors := []string{}
		for _, a := range entry.Authors {
			authors = append(authors, a.Name)
		}
		if len(authors) > 0 {
			text += " Authors: " + strings.Join(authors[:min(3, len(authors))], ", ")
			if len(authors) > 3 {
				text += " et al."
			}
		}

		doc := KorelDoc{
			URL:         entry.ID,
			Title:       cleanText(entry.Title),
			PublishedAt: pubTime,
			Outlet:      "arxiv.org",
			Text:        text,
			SourceCats:  deduplicate(cats),
		}

		if err := encoder.Encode(doc); err != nil {
			log.Printf("Failed to encode doc: %v", err)
			continue
		}

		downloaded++
		if downloaded%25 == 0 {
			log.Printf("Processed %d/%d papers...", downloaded, len(feed.Entries))
		}
	}

	log.Printf("✓ Successfully downloaded %d papers to %s/docs.jsonl", downloaded, outDir)
	log.Println("\nCategories found:", getCategoryStats(feed.Entries))
}

func mapArxivCategory(cat string) string {
	// Map arXiv categories to Korel categories
	mapping := map[string]string{
		"cs.AI": "ai",
		"cs.CL": "nlp",
		"cs.LG": "machine-learning",
		"cs.CV": "computer-vision",
		"cs.CR": "security",
		"cs.DB": "database",
		"cs.SE": "software-engineering",
		"econ.EM": "economics",
		"q-fin": "finance",
		"stat.ML": "statistics",
		"math.OC": "optimization",
		"physics": "physics",
	}

	for prefix, category := range mapping {
		if strings.HasPrefix(cat, prefix) {
			return category
		}
	}

	// Extract major category (before dot)
	parts := strings.Split(cat, ".")
	if len(parts) > 0 {
		return parts[0]
	}

	return "research"
}

func cleanText(s string) string {
	// Remove extra whitespace
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimSpace(s)
	return s
}

func deduplicate(strs []string) []string {
	seen := make(map[string]struct{})
	result := []string{}
	for _, s := range strs {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

func getCategoryStats(entries []ArxivEntry) map[string]int {
	stats := make(map[string]int)
	for _, e := range entries {
		for _, cat := range e.Category {
			mapped := mapArxivCategory(cat.Term)
			stats[mapped]++
		}
	}
	return stats
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
