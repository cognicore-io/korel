package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/cognicore/korel/internal/bootstrap"
	"github.com/cognicore/korel/pkg/korel/inference"
	"github.com/cognicore/korel/pkg/korel/store"
)

func runGraph() {
	var (
		dbPath       = flag.String("db", "", "Database path (required)")
		stoplistPath = flag.String("stoplist", "", "Stoplist file (optional)")
		dictPath     = flag.String("dict", "", "Dictionary file (optional)")
		taxonomyPath = flag.String("taxonomy", "", "Taxonomy file (optional)")
	)
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("--db required")
	}

	ctx := context.Background()

	res, err := bootstrap.Run(ctx, bootstrap.Options{
		DBPath:       *dbPath,
		StoplistPath: *stoplistPath,
		DictPath:     *dictPath,
		TaxonomyPath: *taxonomyPath,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	fmt.Println("Korel Graph REPL")
	fmt.Println("================")
	doStats(res.Store, ctx)
	fmt.Println()
	printGraphHelp()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])
		args := strings.TrimSpace(line[len(parts[0]):])

		switch cmd {
		case "help", "?":
			printGraphHelp()

		case "expand":
			if args == "" {
				fmt.Println("Usage: expand <concept>")
				continue
			}
			doExpand(res.Inference, res.Store, ctx, args)

		case "category", "cat":
			if args == "" {
				fmt.Println("Usage: category <concept>")
				continue
			}
			doCategory(res.Inference, args)

		case "siblings":
			if args == "" {
				fmt.Println("Usage: siblings <concept>")
				continue
			}
			doSiblings(res.Inference, args)

		case "neighbors", "nb":
			if args == "" {
				fmt.Println("Usage: neighbors <concept>")
				continue
			}
			doNeighbors(res.Store, ctx, args)

		case "path":
			tokens := strings.SplitN(args, "->", 2)
			if len(tokens) != 2 {
				fmt.Println("Usage: path <concept> -> <concept>")
				continue
			}
			doPath(res.Inference, strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1]))

		case "query", "q":
			if args == "" {
				fmt.Println("Usage: query <relation> <subject> <object>")
				continue
			}
			doQuery(res.Inference, args)

		case "explain":
			tokens := strings.SplitN(args, "->", 2)
			if len(tokens) != 2 {
				fmt.Println("Usage: explain <concept> -> <concept>")
				continue
			}
			from := strings.TrimSpace(tokens[0])
			to := strings.TrimSpace(tokens[1])
			fmt.Println(res.Inference.Explain("same_domain", from, to))
			fmt.Println(res.Inference.Explain("equivalent", from, to))

		case "stats":
			doStats(res.Store, ctx)

		case "exit", "quit":
			fmt.Println("Bye.")
			return

		default:
			// Try as a concept lookup
			doExpand(res.Inference, res.Store, ctx, line)
		}
	}
	fmt.Println()
}

func printGraphHelp() {
	fmt.Println("Commands:")
	fmt.Println("  expand <concept>          \u2014 Prolog expansion + PMI neighbors")
	fmt.Println("  category <concept>        \u2014 What categories does this belong to?")
	fmt.Println("  siblings <concept>        \u2014 Same-domain concepts (via Prolog)")
	fmt.Println("  neighbors <concept>       \u2014 PMI co-occurrence neighbors (from SQLite)")
	fmt.Println("  path <from> -> <to>       \u2014 Find inference chain between concepts")
	fmt.Println("  explain <from> -> <to>    \u2014 Explain why two concepts are related")
	fmt.Println("  query <rel> <subj> <obj>  \u2014 Check if relation holds")
	fmt.Println("  stats                     \u2014 Corpus statistics")
	fmt.Println("  <anything else>           \u2014 Treated as expand")
	fmt.Println("  exit                      \u2014 Quit")
}

func doExpand(eng inference.Engine, st store.Store, ctx context.Context, concept string) {
	prologResults := eng.Expand([]string{concept})
	neighbors, _ := st.TopNeighbors(ctx, concept, 10)

	fmt.Printf("\n=== %s ===\n", concept)

	if len(prologResults) > 0 {
		sort.Strings(prologResults)
		fmt.Printf("\nProlog (same_domain/equivalent/bridge):\n")
		for _, r := range prologResults {
			fmt.Printf("  \u2192 %s\n", r)
		}
	} else {
		fmt.Println("\nProlog: (no category/synonym edges)")
	}

	if len(neighbors) > 0 {
		fmt.Printf("\nPMI neighbors (corpus co-occurrence):\n")
		for _, nb := range neighbors {
			fmt.Printf("  \u2194 %-30s  PMI: %.3f\n", nb.Token, nb.PMI)
		}
	} else {
		fmt.Println("\nPMI: (not found in corpus)")
	}
}

func doCategory(eng inference.Engine, concept string) {
	cats := eng.QueryAll("category", concept)
	isA := eng.QueryAll("is_a", concept)

	fmt.Printf("\n=== Categories for %q ===\n", concept)
	if len(cats) > 0 {
		for _, c := range cats {
			fmt.Printf("  category: %s\n", c)
		}
	}
	if len(isA) > 0 {
		for _, c := range isA {
			fmt.Printf("  is_a: %s\n", c)
		}
	}
	if len(cats) == 0 && len(isA) == 0 {
		fmt.Println("  (none)")
	}
}

func doSiblings(eng inference.Engine, concept string) {
	siblings := eng.QueryAll("same_domain", concept)
	equiv := eng.QueryAll("equivalent", concept)

	fmt.Printf("\n=== Siblings of %q ===\n", concept)
	if len(siblings) > 0 {
		sort.Strings(siblings)
		fmt.Println("\nSame domain:")
		for _, s := range siblings {
			fmt.Printf("  \u2248 %s\n", s)
		}
	}
	if len(equiv) > 0 {
		sort.Strings(equiv)
		fmt.Println("\nEquivalent (via synonyms):")
		for _, s := range equiv {
			fmt.Printf("  = %s\n", s)
		}
	}
	if len(siblings) == 0 && len(equiv) == 0 {
		fmt.Println("  (none found)")
	}
}

func doNeighbors(st store.Store, ctx context.Context, concept string) {
	neighbors, err := st.TopNeighbors(ctx, concept, 20)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("\n=== PMI neighbors of %q ===\n", concept)
	if len(neighbors) == 0 {
		fmt.Println("  (not found in corpus)")
		return
	}
	for _, nb := range neighbors {
		fmt.Printf("  %-30s  PMI: %.3f\n", nb.Token, nb.PMI)
	}
}

func doPath(eng inference.Engine, from, to string) {
	path := eng.FindPath(from, to)
	fmt.Printf("\n=== Path: %s \u2192 %s ===\n", from, to)
	if len(path) == 0 {
		fmt.Println("  (no path found)")
		return
	}
	for i, step := range path {
		fmt.Printf("  %d. %s -[%s]\u2192 %s\n", i+1, step.From, step.Relation, step.To)
	}
}

func doQuery(eng inference.Engine, args string) {
	parts := strings.Fields(args)
	if len(parts) < 3 {
		fmt.Println("Usage: query <relation> <subject> <object>")
		return
	}
	rel := parts[0]
	subj := strings.Join(parts[1:len(parts)-1], " ")
	obj := parts[len(parts)-1]

	result := eng.Query(rel, subj, obj)
	if result {
		fmt.Printf("  \u2713 %s(%s, %s) holds\n", rel, subj, obj)
	} else {
		fmt.Printf("  \u2717 %s(%s, %s) does not hold\n", rel, subj, obj)
	}
}

func doStats(st store.Store, ctx context.Context) {
	edges, _ := st.AllEdges(ctx)
	relCounts := map[string]int{}
	subjects := map[string]struct{}{}
	objects := map[string]struct{}{}
	for _, e := range edges {
		relCounts[e.Relation]++
		subjects[e.Subject] = struct{}{}
		objects[e.Object] = struct{}{}
	}

	fmt.Println("\n=== Graph Statistics ===")
	fmt.Printf("Total edges: %d\n", len(edges))
	fmt.Printf("Unique subjects: %d\n", len(subjects))
	fmt.Printf("Unique objects: %d\n", len(objects))
	fmt.Println("\nBy relation:")
	for rel, cnt := range relCounts {
		fmt.Printf("  %-15s %d\n", rel, cnt)
	}
}
