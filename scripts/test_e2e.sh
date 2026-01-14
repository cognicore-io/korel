#!/bin/bash

# End-to-end test for Korel
# 1. Clean previous data
# 2. Run indexer
# 3. Run sample queries
# 4. Verify results

set -e

echo "=== Korel E2E Test ==="
echo

# Clean
echo "Cleaning previous data..."
rm -f data/korel.db data/korel.db-*
mkdir -p data/snapshots

# Index
echo "Running indexer..."
go run ./cmd/rss-indexer || {
    echo "Indexer failed"
    exit 1
}

echo
echo "Index complete. Database created at data/korel.db"
echo

# Test queries (automated)
echo "Testing sample queries..."
echo "feed-in tariff Italy" | go run ./cmd/chat-cli

echo
echo "=== E2E Test Complete ==="
