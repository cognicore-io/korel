package korel

import (
	"testing"

	"github.com/cognicore/korel/pkg/korel/store"
)

type fakeDictView struct {
	entries map[string]struct{ canonical, category string }
}

func (f *fakeDictView) Lookup(phrase string) (string, string, bool) {
	e, ok := f.entries[phrase]
	if !ok {
		return "", "", false
	}
	return e.canonical, e.category, true
}

func (f *fakeDictView) AllEntries() []store.DictEntryData {
	return nil
}

func TestRewriterSynonymExpansion(t *testing.T) {
	dict := &fakeDictView{entries: map[string]struct{ canonical, category string }{
		"k8s":    {"kubernetes", "tech"},
		"ml":     {"machine_learning", "tech"},
		"golang": {"go", "lang"},
	}}
	rw := NewRewriter(dict)

	rr := rw.Rewrite("k8s security ml")
	if rr.Rewritten != "kubernetes security machine_learning" {
		t.Errorf("got %q, want %q", rr.Rewritten, "kubernetes security machine_learning")
	}
	if len(rr.Applied) != 2 {
		t.Errorf("expected 2 applied rules, got %d", len(rr.Applied))
	}
}

func TestRewriterNoChange(t *testing.T) {
	dict := &fakeDictView{entries: map[string]struct{ canonical, category string }{}}
	rw := NewRewriter(dict)

	rr := rw.Rewrite("unknown query terms")
	if rr.Rewritten != rr.Original {
		t.Errorf("no-op rewrite should preserve original, got %q vs %q", rr.Rewritten, rr.Original)
	}
	if len(rr.Applied) != 0 {
		t.Errorf("no-op rewrite should have 0 applied rules, got %d", len(rr.Applied))
	}
}

func TestRewriterNilDict(t *testing.T) {
	rw := NewRewriter(nil)
	if rw != nil {
		t.Error("NewRewriter(nil) should return nil")
	}
}
