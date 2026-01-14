package config

import (
	"fmt"

	"github.com/cognicore/korel/pkg/korel/ingest"
)

// Loader loads all configuration files and constructs components
type Loader struct {
	StoplistPath string
	DictPath     string
	TaxonomyPath string
	RulesPath    string
}

// Components holds all loaded configuration components
type Components struct {
	Tokenizer *ingest.Tokenizer
	Parser    *ingest.MultiTokenParser
	Taxonomy  *ingest.Taxonomy
	Rules     string
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
	} else {
		comp.Tokenizer = ingest.NewTokenizer([]string{})
	}

	// Load dictionary
	if l.DictPath != "" {
		dict, err := LoadDict(l.DictPath)
		if err != nil {
			return nil, fmt.Errorf("load dictionary: %w", err)
		}
		entries := make([]ingest.DictEntry, len(dict.Entries))
		for i, e := range dict.Entries {
			entries[i] = ingest.DictEntry{
				Canonical: e.Canonical,
				Variants:  e.Variants,
				Category:  e.Category,
			}
		}
		comp.Parser = ingest.NewMultiTokenParser(entries)
	} else {
		comp.Parser = ingest.NewMultiTokenParser([]ingest.DictEntry{})
	}

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
