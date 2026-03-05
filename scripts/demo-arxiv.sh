#!/bin/bash
# Demo script: Download arXiv papers and test Korel
set -e

echo "╔════════════════════════════════════════╗"
echo "║  Korel Demo: arXiv Papers (cs.AI)     ║"
echo "╚════════════════════════════════════════╝"
echo

# Step 1: Download
echo "📥 Step 1: Downloading arXiv papers..."
echo "   Category: cs.AI (Artificial Intelligence)"
echo "   Count: 200 papers"
echo
go run ./cmd/korel download arxiv cs.AI 200
echo

# Step 2: Index (when implemented)
echo "🔨 Step 2: Indexing papers..."
echo "   [TODO: Implement indexer pipeline]"
echo "   Run: go run ./cmd/korel index --data=testdata/arxiv/docs.jsonl"
echo

# Step 3: Query (when implemented)
echo "🔍 Step 3: Try searching..."
echo "   [TODO: Implement search]"
echo "   Run: go run ./cmd/korel search"
echo "   Example queries:"
echo "     - transformer architecture attention"
echo "     - reinforcement learning optimization"
echo "     - state of the art benchmark"
echo

echo "✓ Papers downloaded to testdata/arxiv/docs.jsonl"
echo "✓ Ready for indexing once pipeline is implemented"
echo
echo "💡 Try other categories:"
echo "   ./scripts/demo-arxiv.sh cs.CL    # NLP papers"
echo "   ./scripts/demo-arxiv.sh cs.LG    # Machine Learning"
echo "   ./scripts/demo-arxiv.sh econ.EM  # Economics"
