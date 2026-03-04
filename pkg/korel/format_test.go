package korel

import (
	"strings"
	"testing"
	"time"
)

func makeTestCards() []Card {
	return []Card{
		{
			ID:    "card-1",
			Title: "Kubernetes Security Best Practices",
			Bullets: []string{
				"Use network policies to restrict pod-to-pod traffic",
				"Enable RBAC for cluster access control",
			},
			Sources: []SourceRef{
				{URL: "https://example.com/k8s-sec", Time: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
			},
		},
		{
			ID:    "card-2",
			Title: "Container Runtime Security",
			Bullets: []string{
				"Use read-only root filesystems",
			},
			Sources: []SourceRef{
				{URL: "https://example.com/containers", Time: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)},
			},
		},
	}
}

func TestFormatBriefing(t *testing.T) {
	cards := makeTestCards()
	out := FormatOutput(FormatBrief, cards)

	if !strings.HasPrefix(out, "BRIEFING\n") {
		t.Error("briefing should start with BRIEFING header")
	}
	if !strings.Contains(out, "1. Kubernetes Security") {
		t.Error("briefing should have numbered cards")
	}
	if !strings.Contains(out, "2. Container Runtime") {
		t.Error("briefing should list all cards")
	}
}

func TestFormatDigest(t *testing.T) {
	cards := makeTestCards()
	out := FormatOutput(FormatDigest, cards)

	if !strings.HasPrefix(out, "DIGEST\n") {
		t.Error("digest should start with DIGEST header")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Header line + separator line + 2 cards = 4 lines
	bulletLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "* ") {
			bulletLines++
		}
	}
	if bulletLines != 2 {
		t.Errorf("digest should have 2 bullet lines, got %d", bulletLines)
	}
}

func TestFormatMemo(t *testing.T) {
	cards := makeTestCards()
	out := FormatOutput(FormatMemo, cards)

	if !strings.HasPrefix(out, "MEMO\n") {
		t.Error("memo should start with MEMO header")
	}
	if !strings.Contains(out, "## Kubernetes Security") {
		t.Error("memo should have section headers")
	}
	if !strings.Contains(out, "Sources:") {
		t.Error("memo should list sources")
	}
}

func TestFormatWatchlist(t *testing.T) {
	cards := makeTestCards()
	out := FormatOutput(FormatWatch, cards)

	if !strings.HasPrefix(out, "WATCHLIST\n") {
		t.Error("watchlist should start with WATCHLIST header")
	}
	if !strings.Contains(out, "TOPIC") {
		t.Error("watchlist should have column headers")
	}
}

func TestFormatCardsReturnsEmpty(t *testing.T) {
	out := FormatOutput(FormatCards, makeTestCards())
	if out != "" {
		t.Error("FormatCards should return empty string (use structured cards)")
	}
}
