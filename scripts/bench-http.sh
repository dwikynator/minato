#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/benchmarks/http"
OUT_FILE="$OUT_DIR/latest.txt"

mkdir -p "$OUT_DIR"

echo "Running HTTP benchmark suite..."
go test -run=^$ -bench '^BenchmarkHTTP$' -benchmem -count=10 ./... | tee "$OUT_FILE"

echo
echo "Raw benchmark output saved to: $OUT_FILE"
if command -v benchstat >/dev/null 2>&1; then
  echo "Tip: benchstat $OUT_FILE"
else
  echo "Tip: install benchstat (golang.org/x/perf/cmd/benchstat) for easier result summaries"
fi
