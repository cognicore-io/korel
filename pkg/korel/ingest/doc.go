package ingest

import (
	"errors"
	"strings"
	"time"
)

// Doc represents a normalized document after extraction
type Doc struct {
	URL         string
	Title       string
	Outlet      string
	PublishedAt time.Time
	BodyText    string
	LinksOut    []string
	SourceCats  []string // from feed configuration
}

// Validate checks if the document has required fields
func (d *Doc) Validate() error {
	if strings.TrimSpace(d.URL) == "" {
		return errors.New("doc URL is required")
	}

	if strings.TrimSpace(d.Title) == "" {
		return errors.New("doc title is required")
	}

	if d.PublishedAt.IsZero() {
		return errors.New("doc published time is required")
	}

	if strings.TrimSpace(d.BodyText) == "" {
		return errors.New("doc body text is required")
	}

	return nil
}
