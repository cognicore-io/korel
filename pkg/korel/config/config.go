package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Taxonomy represents the taxonomy configuration
type Taxonomy struct {
	Sectors  map[string][]string            `yaml:"sectors"`
	Events   map[string][]string            `yaml:"events"`
	Regions  map[string][]string            `yaml:"regions"`
	Entities map[string]map[string][]string `yaml:"entities"`
}

// LoadTaxonomy loads taxonomy from a YAML file
func LoadTaxonomy(path string) (*Taxonomy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tax Taxonomy
	if err := yaml.Unmarshal(data, &tax); err != nil {
		return nil, err
	}

	return &tax, nil
}

// Stoplist represents the stopword list configuration
type Stoplist struct {
	Terms []string `yaml:"terms"`
}

// LoadStoplist loads stopwords from a YAML file
func LoadStoplist(path string) (*Stoplist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sl Stoplist
	if err := yaml.Unmarshal(data, &sl); err != nil {
		return nil, err
	}

	return &sl, nil
}

// Dict represents the multi-token dictionary
type Dict struct {
	Entries []DictEntry
}

// DictEntry represents a dictionary entry
type DictEntry struct {
	Canonical string
	Variants  []string
	Category  string
}

// LoadDict loads the multi-token dictionary from a file
// Format: canonical|variant1|variant2|category
func LoadDict(path string) (*Dict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dict := &Dict{Entries: []DictEntry{}}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		// Trim all parts
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		entry := DictEntry{
			Canonical: parts[0],
			Variants:  parts[1 : len(parts)-1],
			Category:  parts[len(parts)-1],
		}

		dict.Entries = append(dict.Entries, entry)
	}

	return dict, nil
}
