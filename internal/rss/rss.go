package rss

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Item represents a simplified RSS/news item
type Item struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Outlet      string    `json:"outlet"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"text"`
	SourceCats  []string  `json:"source_cats"`
}

// LoadFromJSONL loads items from a JSONL file with proper error handling
func LoadFromJSONL(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	var items []Item
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var item Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			log.Printf("Warning: skipping malformed JSON at line %d in %s: %v", i+1, path, err)
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no valid items found in %s", path)
	}

	return items, nil
}
