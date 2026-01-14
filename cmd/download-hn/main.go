package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Hacker News API endpoints
const (
	apiBase       = "https://hacker-news.firebaseio.com/v0"
	topStoriesURL = apiBase + "/topstories.json"
	itemURL       = apiBase + "/item/%d.json"
)

// HNItem represents a Hacker News story or comment
type HNItem struct {
	ID          int64    `json:"id"`
	Type        string   `json:"type"`
	By          string   `json:"by"`
	Time        int64    `json:"time"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Text        string   `json:"text"`
	Score       int      `json:"score"`
	Descendants int      `json:"descendants"`
	Kids        []int64  `json:"kids"`
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
	count := 100 // Download top 100 stories
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &count)
	}

	log.Printf("Downloading top %d Hacker News stories...\n", count)

	// Get top story IDs
	storyIDs, err := getTopStories()
	if err != nil {
		log.Fatal("Failed to get top stories:", err)
	}

	if count > len(storyIDs) {
		count = len(storyIDs)
	}
	storyIDs = storyIDs[:count]

	// Create output directory
	if err := os.MkdirAll("testdata/hn", 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	// Open output file
	outFile, err := os.Create("testdata/hn/docs.jsonl")
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	downloaded := 0

	// Fetch each story
	for i, id := range storyIDs {
		item, err := getItem(id)
		if err != nil {
			log.Printf("Failed to get item %d: %v", id, err)
			continue
		}

		// Only process stories (not comments/polls)
		if item.Type != "story" {
			continue
		}

		// Skip stories without text content
		if item.Title == "" {
			continue
		}

		// Build text body (title + URL + text if available)
		text := item.Title
		if item.Text != "" {
			text += ". " + stripHTML(item.Text)
		}
		if item.URL != "" {
			text += " [Source: " + item.URL + "]"
		}

		// Categorize based on title/URL keywords
		cats := categorize(item.Title, item.URL)

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

		// Be nice to the API
		time.Sleep(50 * time.Millisecond)
	}

	log.Printf("✓ Successfully downloaded %d stories to testdata/hn/docs.jsonl", downloaded)
}

func getTopStories() ([]int64, error) {
	resp, err := http.Get(topStoriesURL)
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

func getItem(id int64) (*HNItem, error) {
	url := fmt.Sprintf(itemURL, id)
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

	var item HNItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}

	return &item, nil
}

func categorize(title, url string) []string {
	lower := strings.ToLower(title + " " + url)
	cats := []string{}

	// Tech categories
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
		// Fallback to string if parsing fails
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
