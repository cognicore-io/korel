package config

import (
	"fmt"
	"strings"

	"github.com/cognicore/korel/pkg/korel/ingest"
)

// Loader loads all configuration files and constructs components
type Loader struct {
	StoplistPath string
	DictPath     string
	BaseDictPath string // Optional base dictionary merged under DictPath entries
	TaxonomyPath string
	RulesPath    string
}

// Components holds all loaded configuration components
type Components struct {
	Tokenizer *ingest.Tokenizer
	Parser    *ingest.MultiTokenParser
	Taxonomy  *ingest.Taxonomy
	Rules     string
	Stopwords []string // raw stopword list for persistence to store
}

// Load reads all configuration files and returns initialized components
func (l *Loader) Load() (*Components, error) {
	comp := &Components{}

	// Load stoplist
	if l.StoplistPath != "" {
		stoplist, err := LoadStoplist(l.StoplistPath)
		if err != nil {
			return nil, fmt.Errorf("load stoplist: %w", err)
		}
		comp.Tokenizer = ingest.NewTokenizer(stoplist.Terms)
		comp.Stopwords = stoplist.Terms
	} else {
		comp.Tokenizer = ingest.NewTokenizer([]string{})
	}

	// Load dictionary (merge base + domain-specific; domain overrides base)
	var allConfigEntries []DictEntry
	if l.BaseDictPath != "" {
		baseDict, err := LoadDict(l.BaseDictPath)
		if err != nil {
			return nil, fmt.Errorf("load base dictionary: %w", err)
		}
		allConfigEntries = append(allConfigEntries, baseDict.Entries...)
	}
	if l.DictPath != "" {
		dict, err := LoadDict(l.DictPath)
		if err != nil {
			return nil, fmt.Errorf("load dictionary: %w", err)
		}
		allConfigEntries = append(allConfigEntries, dict.Entries...)
	}
	entries := deduplicateDictEntries(allConfigEntries)
	comp.Parser = ingest.NewMultiTokenParser(entries)

	// Load taxonomy
	if l.TaxonomyPath != "" {
		taxConfig, err := LoadTaxonomy(l.TaxonomyPath)
		if err != nil {
			return nil, fmt.Errorf("load taxonomy: %w", err)
		}
		comp.Taxonomy = ingest.NewTaxonomy()
		for name, keywords := range taxConfig.Sectors {
			comp.Taxonomy.AddSector(name, keywords)
		}
		for name, keywords := range taxConfig.Events {
			comp.Taxonomy.AddEvent(name, keywords)
		}
		for name, keywords := range taxConfig.Regions {
			comp.Taxonomy.AddRegion(name, keywords)
		}
		for entityType, entities := range taxConfig.Entities {
			for name, keywords := range entities {
				comp.Taxonomy.AddEntity(entityType, name, keywords)
			}
		}
	} else {
		comp.Taxonomy = ingest.NewTaxonomy()
	}

	// Load rules (just read the file, parsing happens in inference engine)
	if l.RulesPath != "" {
		// Rules are loaded separately by the inference engine
		comp.Rules = l.RulesPath
	}

	return comp, nil
}

// deduplicateDictEntries merges entries by canonical key (case-insensitive).
// Later entries override earlier ones, so domain-specific entries win over base.
func deduplicateDictEntries(cfgEntries []DictEntry) []ingest.DictEntry {
	seen := make(map[string]int) // lowercase canonical → index in result
	var result []ingest.DictEntry
	for _, e := range cfgEntries {
		key := strings.ToLower(e.Canonical)
		ie := ingest.DictEntry{
			Canonical: e.Canonical,
			Variants:  e.Variants,
			Category:  e.Category,
		}
		if idx, ok := seen[key]; ok {
			result[idx] = ie
		} else {
			seen[key] = len(result)
			result = append(result, ie)
		}
	}
	return result
}
