package store

import (
	"context"
	"time"
)

// Store is the main interface for persisting and querying Korel data
type Store interface {
	Close() error

	// Docs
	UpsertDoc(ctx context.Context, d Doc) error
	GetDoc(ctx context.Context, id int64) (Doc, error)
	GetDocByURL(ctx context.Context, url string) (Doc, bool, error)
	GetDocsByTokens(ctx context.Context, tokens []string, limit int) ([]Doc, error)

	// Tokens & Counts
	UpsertTokenDF(ctx context.Context, token string, df int64) error
	GetTokenDF(ctx context.Context, token string) (int64, error)
	IncPair(ctx context.Context, t1, t2 string) error
	DecPair(ctx context.Context, t1, t2 string) error
	GetPMI(ctx context.Context, t1, t2 string) (float64, bool, error)
	TopNeighbors(ctx context.Context, token string, k int) ([]Neighbor, error)

	// Cards
	UpsertCard(ctx context.Context, c Card) error
	GetCardsByPeriod(ctx context.Context, period string, k int) ([]Card, error)

	// Config/Stoplist/Dict/Taxonomy (optional as read-through cache)
	Stoplist() StoplistView
	Dict() DictView
	Taxonomy() TaxonomyView

	// Persistence for AutoTune results
	UpsertStoplist(ctx context.Context, tokens []string) error
	UpsertDictEntry(ctx context.Context, phrase, canonical, category string) error
}

// Doc represents a stored document
type Doc struct {
	ID          int64
	URL         string
	Title       string
	Outlet      string
	PublishedAt time.Time
	Cats        []string
	Ents        []Entity
	LinksOut    int
	Tokens      []string
}

// Entity represents a recognized entity in a document
type Entity struct {
	Type  string // TICKER, COUNTRY, DATE, etc.
	Value string
}

// Neighbor represents a token's PMI neighbor
type Neighbor struct {
	Token string
	PMI   float64
}

// Card represents a stored result card
type Card struct {
	ID             string
	Title          string
	Bullets        []string
	Sources        []string // JSON-encoded SourceRefs
	ScoreJSON      string
	Period         string
}

// StoplistView provides read access to the stopword list
type StoplistView interface {
	IsStop(token string) bool
	AllStops() []string
}

// DictEntryData holds a single dictionary entry for iteration.
type DictEntryData struct {
	Phrase    string
	Canonical string
	Category  string
}

// DictView provides read access to the multi-token dictionary
type DictView interface {
	Lookup(phrase string) (canonical string, category string, ok bool)
	AllEntries() []DictEntryData
}

// TaxonomyView provides read access to the taxonomy
type TaxonomyView interface {
	CategoriesForToken(token string) []string
	EntitiesInText(text string) []Entity
	AllSectors() map[string][]string
	AllEvents() map[string][]string
	AllRegions() map[string][]string
	AllEntities() map[string]map[string][]string
}
