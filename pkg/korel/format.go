package korel

import (
	"fmt"
	"strings"
)

// FormatOutput converts cards into the specified output format.
// Returns empty string for FormatCards (the default structured card output).
func FormatOutput(format OutputFormat, cards []Card) string {
	switch format {
	case FormatBrief:
		return formatBriefing(cards)
	case FormatMemo:
		return formatMemo(cards)
	case FormatDigest:
		return formatDigest(cards)
	case FormatWatch:
		return formatWatchlist(cards)
	default:
		return ""
	}
}

func formatBriefing(cards []Card) string {
	var b strings.Builder
	b.WriteString("BRIEFING\n========\n\n")
	for i, card := range cards {
		fmt.Fprintf(&b, "%d. %s\n", i+1, card.Title)
		for _, bullet := range card.Bullets {
			fmt.Fprintf(&b, "   - %s\n", bullet)
		}
		if len(card.Sources) > 0 {
			fmt.Fprintf(&b, "   Source: %s (%s)\n", card.Sources[0].URL, card.Sources[0].Time.Format("2006-01-02"))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatMemo(cards []Card) string {
	var b strings.Builder
	b.WriteString("MEMO\n====\n\n")
	for _, card := range cards {
		fmt.Fprintf(&b, "## %s\n\n", card.Title)
		for _, bullet := range card.Bullets {
			fmt.Fprintf(&b, "- %s\n", bullet)
		}
		b.WriteString("\nSources:\n")
		for _, src := range card.Sources {
			fmt.Fprintf(&b, "  - %s (%s)\n", src.URL, src.Time.Format("2006-01-02"))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatDigest(cards []Card) string {
	var b strings.Builder
	b.WriteString("DIGEST\n------\n")
	for _, card := range cards {
		summary := card.Title
		if len(card.Bullets) > 0 {
			summary += " — " + card.Bullets[0]
		}
		fmt.Fprintf(&b, "* %s\n", summary)
	}
	return b.String()
}

func formatWatchlist(cards []Card) string {
	var b strings.Builder
	b.WriteString("WATCHLIST\n---------\n")
	fmt.Fprintf(&b, "%-30s %-12s %s\n", "TOPIC", "DATE", "SIGNAL")
	fmt.Fprintf(&b, "%-30s %-12s %s\n", strings.Repeat("-", 30), strings.Repeat("-", 12), strings.Repeat("-", 20))
	for _, card := range cards {
		date := ""
		if len(card.Sources) > 0 {
			date = card.Sources[0].Time.Format("2006-01-02")
		}
		signal := ""
		if len(card.Bullets) > 0 {
			signal = fmtTruncate(card.Bullets[0], 40)
		}
		title := fmtTruncate(card.Title, 30)
		fmt.Fprintf(&b, "%-30s %-12s %s\n", title, date, signal)
	}
	return b.String()
}

func fmtTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
