package ingest

import "strings"

// Taxonomy handles categorization and entity extraction
type Taxonomy struct {
	sectors  map[string][]string // category → keywords (lowercase)
	events   map[string][]string
	regions  map[string][]string
	entities map[string]map[string][]string // type → name → keywords
}

// NewTaxonomy creates a new taxonomy from configuration
func NewTaxonomy() *Taxonomy {
	return &Taxonomy{
		sectors:  make(map[string][]string),
		events:   make(map[string][]string),
		regions:  make(map[string][]string),
		entities: make(map[string]map[string][]string),
	}
}

// AddSector adds a sector category with its keywords
func (t *Taxonomy) AddSector(name string, keywords []string) {
	normalized := make([]string, len(keywords))
	for i, kw := range keywords {
		normalized[i] = strings.ToLower(kw)
	}
	t.sectors[name] = normalized
}

// AddEvent adds an event category with its keywords
func (t *Taxonomy) AddEvent(name string, keywords []string) {
	normalized := make([]string, len(keywords))
	for i, kw := range keywords {
		normalized[i] = strings.ToLower(kw)
	}
	t.events[name] = normalized
}

// AddRegion adds a region category with its keywords
func (t *Taxonomy) AddRegion(name string, keywords []string) {
	normalized := make([]string, len(keywords))
	for i, kw := range keywords {
		normalized[i] = strings.ToLower(kw)
	}
	t.regions[name] = normalized
}

// AddEntity adds an entity type with name and keywords
func (t *Taxonomy) AddEntity(entityType, name string, keywords []string) {
	if t.entities[entityType] == nil {
		t.entities[entityType] = make(map[string][]string)
	}
	t.entities[entityType][name] = keywords
}

// Entity represents a recognized entity
type Entity struct {
	Type  string
	Value string
}

// AssignCategories determines which categories apply to the given tokens
func (t *Taxonomy) AssignCategories(tokens []string) []string {
	cats := make(map[string]struct{})

	// Normalize tokens to lowercase for case-insensitive matching
	tokenSet := make(map[string]struct{})
	for _, tok := range tokens {
		tokenSet[strings.ToLower(tok)] = struct{}{}
	}

	// Check sectors
	for cat, keywords := range t.sectors {
		for _, kw := range keywords {
			if _, ok := tokenSet[kw]; ok {
				cats[cat] = struct{}{}
				break
			}
		}
	}

	// Check events
	for cat, keywords := range t.events {
		for _, kw := range keywords {
			if _, ok := tokenSet[kw]; ok {
				cats[cat] = struct{}{}
				break
			}
		}
	}

	// Check regions
	for cat, keywords := range t.regions {
		for _, kw := range keywords {
			if _, ok := tokenSet[kw]; ok {
				cats[cat] = struct{}{}
				break
			}
		}
	}

	result := make([]string, 0, len(cats))
	for cat := range cats {
		result = append(result, cat)
	}
	return result
}

// ExtractEntities finds entities in the text
func (t *Taxonomy) ExtractEntities(text string) []Entity {
	var entities []Entity
	lowerText := strings.ToLower(text)

	// Match entities by keywords
	for entityType, namedEntities := range t.entities {
		for name, keywords := range namedEntities {
			for _, kw := range keywords {
				if strings.Contains(lowerText, strings.ToLower(kw)) {
					entities = append(entities, Entity{
						Type:  entityType,
						Value: name,
					})
					break
				}
			}
		}
	}

	return entities
}
