package memstore

import (
	"testing"
)

func TestStoplist_NilWhenUnconfigured(t *testing.T) {
	s := New()
	if s.Stoplist() != nil {
		t.Fatal("expected nil Stoplist when unconfigured")
	}
}

func TestStoplist_SetAndQuery(t *testing.T) {
	s := New()
	s.SetStoplist([]string{"the", "a", "and"})

	sl := s.Stoplist()
	if sl == nil {
		t.Fatal("expected non-nil Stoplist after SetStoplist")
	}

	if !sl.IsStop("the") {
		t.Error("expected 'the' to be a stop word")
	}
	if !sl.IsStop("a") {
		t.Error("expected 'a' to be a stop word")
	}
	if sl.IsStop("machine") {
		t.Error("expected 'machine' NOT to be a stop word")
	}
}

func TestStoplist_AllStops(t *testing.T) {
	s := New()
	s.SetStoplist([]string{"zebra", "apple", "mango"})

	stops := s.Stoplist().AllStops()
	if len(stops) != 3 {
		t.Fatalf("expected 3 stops, got %d", len(stops))
	}
	// Should be sorted
	if stops[0] != "apple" || stops[1] != "mango" || stops[2] != "zebra" {
		t.Errorf("expected sorted [apple mango zebra], got %v", stops)
	}
}

func TestStoplist_ReplacesClearsOld(t *testing.T) {
	s := New()
	s.SetStoplist([]string{"old"})
	s.SetStoplist([]string{"new"})

	sl := s.Stoplist()
	if sl.IsStop("old") {
		t.Error("old stopword should be gone after replace")
	}
	if !sl.IsStop("new") {
		t.Error("new stopword should be present")
	}
}

func TestDict_NilWhenUnconfigured(t *testing.T) {
	s := New()
	if s.Dict() != nil {
		t.Fatal("expected nil Dict when unconfigured")
	}
}

func TestDict_AddAndLookup(t *testing.T) {
	s := New()
	s.AddDictEntry("ml", "machine learning", "ai")
	s.AddDictEntry("nn", "neural network", "ai")

	d := s.Dict()
	if d == nil {
		t.Fatal("expected non-nil Dict after AddDictEntry")
	}

	canonical, cat, ok := d.Lookup("ml")
	if !ok {
		t.Fatal("expected 'ml' to be found")
	}
	if canonical != "machine learning" {
		t.Errorf("expected canonical 'machine learning', got %q", canonical)
	}
	if cat != "ai" {
		t.Errorf("expected category 'ai', got %q", cat)
	}

	_, _, ok = d.Lookup("unknown")
	if ok {
		t.Error("expected 'unknown' NOT to be found")
	}
}

func TestDict_Overwrite(t *testing.T) {
	s := New()
	s.AddDictEntry("ml", "machine learning", "ai")
	s.AddDictEntry("ml", "markup language", "web")

	canonical, cat, ok := s.Dict().Lookup("ml")
	if !ok {
		t.Fatal("expected 'ml' to be found")
	}
	if canonical != "markup language" || cat != "web" {
		t.Errorf("expected overwritten entry, got %q / %q", canonical, cat)
	}
}

func TestTaxonomy_NilWhenUnconfigured(t *testing.T) {
	s := New()
	if s.Taxonomy() != nil {
		t.Fatal("expected nil Taxonomy when unconfigured")
	}
}

func TestTaxonomy_CategoriesForToken(t *testing.T) {
	s := New()
	s.SetTaxonomy(
		map[string][]string{"ai": {"machine-learning", "neural-network"}},
		map[string][]string{"release": {"launched", "released"}},
		map[string][]string{"us": {"california", "new-york"}},
		nil,
	)

	tv := s.Taxonomy()
	if tv == nil {
		t.Fatal("expected non-nil Taxonomy")
	}

	cats := tv.CategoriesForToken("machine-learning")
	if len(cats) != 1 || cats[0] != "ai" {
		t.Errorf("expected [ai], got %v", cats)
	}

	cats = tv.CategoriesForToken("launched")
	if len(cats) != 1 || cats[0] != "release" {
		t.Errorf("expected [release], got %v", cats)
	}

	cats = tv.CategoriesForToken("unknown")
	if len(cats) != 0 {
		t.Errorf("expected no categories for 'unknown', got %v", cats)
	}
}

func TestTaxonomy_CaseInsensitive(t *testing.T) {
	s := New()
	s.SetTaxonomy(
		map[string][]string{"ai": {"Machine-Learning"}},
		nil, nil, nil,
	)

	cats := s.Taxonomy().CategoriesForToken("machine-learning")
	if len(cats) != 1 || cats[0] != "ai" {
		t.Errorf("expected case-insensitive match [ai], got %v", cats)
	}
}

func TestTaxonomy_EntitiesInText(t *testing.T) {
	s := New()
	s.SetTaxonomy(nil, nil, nil,
		map[string]map[string][]string{
			"company": {
				"Tesla":  {"tesla", "tsla"},
				"Google": {"google", "alphabet"},
			},
		},
	)

	ents := s.Taxonomy().EntitiesInText("Tesla stock is rising and google is expanding")
	if len(ents) != 2 {
		t.Fatalf("expected 2 entities, got %d: %v", len(ents), ents)
	}

	found := map[string]bool{}
	for _, e := range ents {
		found[e.Value] = true
		if e.Type != "company" {
			t.Errorf("expected type 'company', got %q", e.Type)
		}
	}
	if !found["Tesla"] || !found["Google"] {
		t.Errorf("expected Tesla and Google, got %v", ents)
	}
}

func TestTaxonomy_NoEntitiesMatch(t *testing.T) {
	s := New()
	s.SetTaxonomy(nil, nil, nil,
		map[string]map[string][]string{
			"company": {"Tesla": {"tesla"}},
		},
	)

	ents := s.Taxonomy().EntitiesInText("no companies mentioned here")
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %v", ents)
	}
}
