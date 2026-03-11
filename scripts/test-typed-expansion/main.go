// test-typed-expansion validates the reltype classifier against TaxMind's
// korel.db. Focused diagnostic: checks known pairs and shows detailed
// classifier signals.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"text/tabwriter"

	"github.com/cognicore/korel/pkg/korel/reltype"
	"github.com/cognicore/korel/pkg/korel/store"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

func main() {
	dbPath := flag.String("db", "", "path to korel.db")
	topK := flag.Int("topk", 20, "top-K neighbors for comparison")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: test-typed-expansion -db <path>")
		os.Exit(1)
	}

	ctx := context.Background()

	st, err := sqlite.OpenSQLite(ctx, *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	totalDocs, _ := st.DocCount(ctx)
	fmt.Printf("Corpus: %d documents\n\n", totalDocs)

	cfg := reltype.DefaultConfig()
	cfg.TopK = *topK
	cfg.MinConfidence = 0.15
	classifier := reltype.NewClassifier(cfg)

	// === Part 1: Known pairs ===
	pairs := []struct {
		a, b     string
		expected string
		note     string
	}{
		// TaxMind manual synonyms
		{"abschreibung", "afa", "same_as", "manual synonym"},
		{"ehepartner", "ehegatte", "same_as", "manual synonym"},
		{"verlustverrechnung", "verlustabzug", "same_as", "manual synonym"},
		{"homeoffice", "arbeitszimmer", "same_as", "manual synonym"},
		{"umsatzsteuer", "mehrwertsteuer", "same_as", "manual synonym"},

		// Broader/narrower candidates (both terms in corpus)
		{"vorsteuer", "vorsteuerabzug", "broader", "specific application"},
		{"einkommensteuer", "lohnsteuer", "broader", "type of income tax"},
		{"freibetrag", "grundfreibetrag", "broader", "specific freibetrag"},
		{"kapitalgesellschaft", "aktiengesellschaft", "broader", "AG is a type"},
		{"steuerpflicht", "steuerpflichtige", "related", "inflection"},

		// Terms that should be related but not synonyms
		{"einkommensteuer", "körperschaftsteuer", "related", "sibling tax types"},
		{"vorsteuer", "umsatzsteuer", "related", "related within USt domain"},
		{"gewinn", "verlust", "related", "complementary concepts"},
		{"bilanz", "jahresabschluss", "broader", "bilanz is part of jahresabschluss"},
		{"erbe", "erbschaft", "same_as", "same root concept"},
		{"schenkung", "erbschaft", "related", "sibling under ErbStG"},
		{"verschmelzung", "umwandlung", "narrower", "verschmelzung is a type of umwandlung"},
	}

	fmt.Println("=== Pair Analysis ===")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "PAIR\tEXP\tGOT\tCONF\tDFA\tDFB\tPMI\tCOAB\tNOTE\n")
	fmt.Fprintf(w, "----\t---\t---\t----\t---\t---\t---\t----\t----\n")

	for _, p := range pairs {
		dfA, _ := st.GetTokenDF(ctx, p.a)
		dfB, _ := st.GetTokenDF(ctx, p.b)
		pmiScore, _, _ := st.GetPMI(ctx, p.a, p.b)

		neighborsA, _ := st.TopNeighbors(ctx, p.a, *topK)
		neighborsB, _ := st.TopNeighbors(ctx, p.b, *topK)

		rtA := toRelNeighbors(neighborsA)
		rtB := toRelNeighbors(neighborsB)

		var coAB int64
		if pmiScore > 0 && dfA > 0 && dfB > 0 && totalDocs > 0 {
			coAB = int64(math.Exp(pmiScore) * float64(dfA) * float64(dfB) / float64(totalDocs))
		}

		cl := classifier.Classify(rtA, rtB, dfA, dfB, coAB, totalDocs)

		match := ""
		if string(cl.Type) == p.expected {
			match = " OK"
		}

		fmt.Fprintf(w, "%s / %s\t%s\t%s%s\t%.3f\t%d\t%d\t%.3f\t%d\t%s\n",
			p.a, p.b, p.expected, cl.Type, match, cl.Confidence, dfA, dfB, pmiScore, coAB, p.note)
	}
	w.Flush()

	// === Part 2: Detailed neighbor analysis for interesting pairs ===
	detailPairs := [][2]string{
		{"vorsteuer", "vorsteuerabzug"},
		{"einkommensteuer", "lohnsteuer"},
		{"freibetrag", "grundfreibetrag"},
		{"verschmelzung", "umwandlung"},
		{"einkommensteuer", "körperschaftsteuer"},
	}

	for _, dp := range detailPairs {
		a, b := dp[0], dp[1]
		fmt.Printf("\n--- Detail: %s vs %s ---\n", a, b)

		neighborsA, _ := st.TopNeighbors(ctx, a, *topK)
		neighborsB, _ := st.TopNeighbors(ctx, b, *topK)

		dfA, _ := st.GetTokenDF(ctx, a)
		dfB, _ := st.GetTokenDF(ctx, b)

		fmt.Printf("  DF: %s=%d, %s=%d\n", a, dfA, b, dfB)

		setA := make(map[string]float64)
		for _, n := range neighborsA {
			setA[n.Token] = n.PMI
		}
		setB := make(map[string]float64)
		for _, n := range neighborsB {
			setB[n.Token] = n.PMI
		}

		// Overlap
		var overlap []string
		for tok := range setA {
			if _, ok := setB[tok]; ok {
				overlap = append(overlap, tok)
			}
		}

		onlyA := len(setA) - len(overlap)
		onlyB := len(setB) - len(overlap)
		union := len(setA) + len(setB) - len(overlap)
		jaccard := 0.0
		if union > 0 {
			jaccard = float64(len(overlap)) / float64(union)
		}

		fmt.Printf("  Neighbors: %s=%d, %s=%d\n", a, len(setA), b, len(setB))
		fmt.Printf("  Overlap: %d, Only-A: %d, Only-B: %d\n", len(overlap), onlyA, onlyB)
		fmt.Printf("  Jaccard: %.3f\n", jaccard)

		// Weeds precision
		wpAB := float64(len(overlap)) / float64(max(len(setA), 1))
		wpBA := float64(len(overlap)) / float64(max(len(setB), 1))
		fmt.Printf("  Weeds(A→B): %.3f  Weeds(B→A): %.3f\n", wpAB, wpBA)

		if len(overlap) > 0 {
			fmt.Printf("  Shared: ")
			limit := 10
			if len(overlap) < limit {
				limit = len(overlap)
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("%s ", overlap[i])
			}
			fmt.Println()
		}

		// Show top-5 unique to each
		fmt.Printf("  Only in %s (top 5): ", a)
		count := 0
		for _, n := range neighborsA {
			if _, ok := setB[n.Token]; !ok {
				fmt.Printf("%s(%.2f) ", n.Token, n.PMI)
				count++
				if count >= 5 {
					break
				}
			}
		}
		fmt.Println()

		fmt.Printf("  Only in %s (top 5): ", b)
		count = 0
		for _, n := range neighborsB {
			if _, ok := setA[n.Token]; !ok {
				fmt.Printf("%s(%.2f) ", n.Token, n.PMI)
				count++
				if count >= 5 {
					break
				}
			}
		}
		fmt.Println()
	}
}

func toRelNeighbors(neighbors []store.Neighbor) []reltype.Neighbor {
	result := make([]reltype.Neighbor, len(neighbors))
	for i, n := range neighbors {
		result[i] = reltype.Neighbor{Token: n.Token, PMI: n.PMI}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
