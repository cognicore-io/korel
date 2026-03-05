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

	"golang.org/x/net/html"
)

// KorelDoc is the format for Korel ingestion.
type KorelDoc struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
	Outlet      string    `json:"outlet"`
	Text        string    `json:"text"`
	SourceCats  []string  `json:"source_cats"`
}

func runDownload() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: korel download <source> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Sources:")
		fmt.Fprintln(os.Stderr, "  hn [count]                Download top Hacker News stories (default: 100)")
		fmt.Fprintln(os.Stderr, "  arxiv [category] [count]  Download arXiv papers (default: cs.AI, 200)")
		os.Exit(1)
	}

	source := strings.ToLower(os.Args[1])
	os.Args = os.Args[1:] // shift for source-specific arg parsing

	switch source {
	case "hn":
		downloadHN()
	case "arxiv":
		downloadArxiv()
	default:
		fmt.Fprintf(os.Stderr, "unknown download source: %s (use 'hn' or 'arxiv')\n", source)
		os.Exit(1)
	}
}

// ---------- Hacker News ----------

const (
	hnAPIBase       = "https://hacker-news.firebaseio.com/v0"
	hnTopStoriesURL = hnAPIBase + "/topstories.json"
	hnItemURL       = hnAPIBase + "/item/%d.json"
)

type hnItem struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	By          string  `json:"by"`
	Time        int64   `json:"time"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Text        string  `json:"text"`
	Score       int     `json:"score"`
	Descendants int     `json:"descendants"`
	Kids        []int64 `json:"kids"`
}

func downloadHN() {
	count := 100
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &count)
	}

	log.Printf("Downloading top %d Hacker News stories...\n", count)

	storyIDs, err := getTopStories()
	if err != nil {
		log.Fatal("Failed to get top stories:", err)
	}

	if count > len(storyIDs) {
		count = len(storyIDs)
	}
	storyIDs = storyIDs[:count]

	if err := os.MkdirAll("testdata/hn", 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	outFile, err := os.Create("testdata/hn/docs.jsonl")
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	downloaded := 0

	for i, id := range storyIDs {
		item, err := getHNItem(id)
		if err != nil {
			log.Printf("Failed to get item %d: %v", id, err)
			continue
		}

		if item.Type != "story" {
			continue
		}

		if item.Title == "" {
			continue
		}

		text := item.Title
		if item.Text != "" {
			text += ". " + stripHTML(item.Text)
		}
		if item.URL != "" {
			text += " [Source: " + item.URL + "]"
		}

		cats := categorizeHN(item.Title, item.URL)

		doc := KorelDoc{
			URL:         fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID),
			Title:       item.Title,
			PublishedAt: time.Unix(item.Time, 0),
			Outlet:      "news.ycombinator.com",
			Text:        text,
			SourceCats:  cats,
		}

		if err := encoder.Encode(doc); err != nil {
			log.Printf("Failed to encode doc: %v", err)
			continue
		}

		downloaded++
		if (i+1)%10 == 0 {
			log.Printf("Downloaded %d/%d stories...", downloaded, count)
		}

		time.Sleep(50 * time.Millisecond)
	}

	log.Printf("Successfully downloaded %d stories to testdata/hn/docs.jsonl", downloaded)
}

func getTopStories() ([]int64, error) {
	resp, err := http.Get(hnTopStoriesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func getHNItem(id int64) (*hnItem, error) {
	url := fmt.Sprintf(hnItemURL, id)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var item hnItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func categorizeHN(title, url string) []string {
	lower := strings.ToLower(title + " " + url)
	cats := []string{}

	if containsAny(lower, "ai", "llm", "gpt", "machine learning", "neural") {
		cats = append(cats, "ai")
	}
	if containsAny(lower, "startup", "funding", "series a", "venture", "vc") {
		cats = append(cats, "startup")
	}
	if containsAny(lower, "programming", "code", "developer", "framework", "library") {
		cats = append(cats, "programming")
	}
	if containsAny(lower, "security", "vulnerability", "breach", "hack", "crypto") {
		cats = append(cats, "security")
	}
	if containsAny(lower, "web", "browser", "chrome", "firefox", "html", "css") {
		cats = append(cats, "web")
	}
	if containsAny(lower, "open source", "oss", "github", "license") {
		cats = append(cats, "opensource")
	}

	if len(cats) == 0 {
		cats = append(cats, "tech")
	}
	return cats
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func stripHTML(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}

	var buf strings.Builder
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}
	extractText(doc)
	return strings.TrimSpace(buf.String())
}

// ---------- arXiv ----------

const arxivAPIURL = "http://export.arxiv.org/api/query"

type arxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []arxivAuthor `xml:"author"`
	Category  []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
	Link []struct {
		Href string `xml:"href,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

func downloadArxiv() {
	category := "cs.AI"
	maxResults := 200

	if len(os.Args) > 1 {
		category = os.Args[1]
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &maxResults)
	}

	log.Printf("Downloading %d papers from arXiv category: %s\n", maxResults, category)
	log.Println("Categories: cs.AI (AI), cs.CL (NLP), cs.LG (ML), econ.EM (Economics), q-fin (Finance)")

	params := url.Values{}
	params.Set("search_query", "cat:"+category)
	params.Set("max_results", fmt.Sprintf("%d", maxResults))
	params.Set("sortBy", "submittedDate")
	params.Set("sortOrder", "descending")

	fullURL := arxivAPIURL + "?" + params.Encode()

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

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		log.Fatal("Failed to parse XML:", err)
	}

	log.Printf("Received %d papers\n", len(feed.Entries))

	outDir := "testdata/arxiv"
	os.MkdirAll(outDir, 0755)

	outFile, err := os.Create(outDir + "/docs.jsonl")
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	downloaded := 0

	for _, entry := range feed.Entries {
		pubTime, err := time.Parse("2006-01-02T15:04:05Z", entry.Published)
		if err != nil {
			pubTime = time.Now()
		}

		cats := []string{}
		for _, cat := range entry.Category {
			cats = append(cats, mapArxivCategory(cat.Term))
		}
		if len(cats) == 0 {
			cats = []string{"research"}
		}

		text := cleanDownloadText(entry.Title) + ". " + cleanDownloadText(entry.Summary)

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
			Title:       cleanDownloadText(entry.Title),
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

	log.Printf("Successfully downloaded %d papers to %s/docs.jsonl", downloaded, outDir)
	log.Println("\nCategories found:", getCategoryStats(feed.Entries))
}

func mapArxivCategory(cat string) string {
	mapping := map[string]string{
		"cs.AI":   "ai",
		"cs.CL":   "nlp",
		"cs.LG":   "machine-learning",
		"cs.CV":   "computer-vision",
		"cs.CR":   "security",
		"cs.DB":   "database",
		"cs.SE":   "software-engineering",
		"econ.EM": "economics",
		"q-fin":   "finance",
		"stat.ML": "statistics",
		"math.OC": "optimization",
		"physics": "physics",
	}

	for prefix, category := range mapping {
		if strings.HasPrefix(cat, prefix) {
			return category
		}
	}

	parts := strings.Split(cat, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "research"
}

func cleanDownloadText(s string) string {
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

func getCategoryStats(entries []arxivEntry) map[string]int {
	stats := make(map[string]int)
	for _, e := range entries {
		for _, cat := range e.Category {
			mapped := mapArxivCategory(cat.Term)
			stats[mapped]++
		}
	}
	return stats
}
