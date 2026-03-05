package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	os.Args = os.Args[1:] // shift so flag.Parse() works per subcommand

	switch cmd {
	case "index":
		runIndex()
	case "search":
		runSearch()
	case "graph":
		runGraph()
	case "analyze":
		runAnalyze()
	case "bootstrap":
		runBootstrap()
	case "download":
		runDownload()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: korel <command> [flags]

Commands:
  index      Ingest JSONL documents into database
  search     Query the knowledge graph (interactive or one-shot)
  graph      Interactive graph exploration REPL
  analyze    Corpus statistics and stopword candidates
  bootstrap  Generate config files from raw corpus
  download   Fetch data from external sources (hn, arxiv)

Run 'korel <command> -h' for command-specific help.`)
}
