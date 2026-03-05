#!/bin/bash
# Demo script: Download HN data and test Korel
set -e

echo "╔════════════════════════════════════════╗"
echo "║  Korel Demo: Hacker News Dataset      ║"
echo "╚════════════════════════════════════════╝"
echo

# Step 1: Download
echo "📥 Step 1: Downloading HN stories..."
go run ./cmd/korel download hn 100
echo

# Step 2: Index (when implemented)
echo "🔨 Step 2: Indexing data..."
echo "   [TODO: Implement indexer pipeline]"
echo "   Run: go run ./cmd/korel index --data=testdata/hn/docs.jsonl"
echo

# Step 3: Query (when implemented)
echo "🔍 Step 3: Try searching..."
echo "   [TODO: Implement search]"
echo "   Run: go run ./cmd/korel search"
echo "   Example queries:"
echo "     - machine learning framework"
echo "     - startup funding"
echo "     - open source llm"
echo

echo "✓ Data downloaded to testdata/hn/docs.jsonl"
echo "✓ Ready for indexing once pipeline is implemented"
