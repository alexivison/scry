#!/usr/bin/env bash
# bench.sh — Measure file-list first-paint time on the simple fixture.
# Target: <500ms for MetadataService.ListFiles on a medium-sized fixture.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Running ListFiles benchmark (fixture setup included in TestMain)..."
cd "$ROOT"
go test -bench=BenchmarkListFiles -benchtime=10x -count=3 -run='^$' ./internal/diff/ 2>&1

echo ""
echo "Target: each ListFiles call should be well under 500ms."
echo "The benchmark iterations (ns/op) above confirm per-call latency."
