package korel

import (
	"testing"
	"time"

	"github.com/cognicore/korel/pkg/korel/rank"
	"github.com/cognicore/korel/pkg/korel/store"
)

func TestScoreEvidenceEmpty(t *testing.T) {
	ev := scoreEvidence(nil, time.Now(), 14.0)
	if ev.Freshness != 0 || ev.Corroboration != 0 || ev.Authority != 0 || ev.Overall != 0 {
		t.Errorf("empty input should give zero scores, got %+v", ev)
	}
}

func TestScoreEvidenceSingleDoc(t *testing.T) {
	now := time.Now()
	docs := []scored{{
		doc: store.Doc{
			PublishedAt: now.Add(-24 * time.Hour), // 1 day old
			Tokens:      []string{"kubernetes", "security"},
			LinksOut:    10,
		},
		breakdown: rank.ScoreBreakdown{},
	}}

	ev := scoreEvidence(docs, now, 14.0)

	if ev.Freshness <= 0 || ev.Freshness > 1.0 {
		t.Errorf("freshness should be in (0,1], got %.4f", ev.Freshness)
	}
	// Single doc → no corroboration possible
	if ev.Corroboration != 0 {
		t.Errorf("single doc should have 0 corroboration, got %.4f", ev.Corroboration)
	}
	if ev.Authority <= 0 || ev.Authority > 1.0 {
		t.Errorf("authority should be in (0,1], got %.4f", ev.Authority)
	}
	if ev.Overall <= 0 {
		t.Errorf("overall should be positive, got %.4f", ev.Overall)
	}
}

func TestScoreEvidenceCorroboration(t *testing.T) {
	now := time.Now()

	// Three docs with overlapping tokens → should have corroboration > 0
	docs := []scored{
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"kubernetes", "security", "network"}, LinksOut: 5}},
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"kubernetes", "security", "pod"}, LinksOut: 3}},
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"kubernetes", "network", "policy"}, LinksOut: 7}},
	}
	ev := scoreEvidence(docs, now, 14.0)

	if ev.Corroboration <= 0 {
		t.Errorf("overlapping tokens should produce corroboration > 0, got %.4f", ev.Corroboration)
	}

	// Three docs with disjoint tokens → lower corroboration
	disjoint := []scored{
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"alpha"}, LinksOut: 1}},
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"beta"}, LinksOut: 1}},
		{doc: store.Doc{PublishedAt: now, Tokens: []string{"gamma"}, LinksOut: 1}},
	}
	evDisjoint := scoreEvidence(disjoint, now, 14.0)

	if evDisjoint.Corroboration >= ev.Corroboration {
		t.Errorf("disjoint tokens should have lower corroboration (%.4f) than overlapping (%.4f)",
			evDisjoint.Corroboration, ev.Corroboration)
	}
}
