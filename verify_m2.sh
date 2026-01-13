#!/bin/bash
# verify_m2.sh - Final verification script for M2 Testing milestone
# Runs all tests, generates coverage reports, and displays results

set -e

cd "$(dirname "$0")"

echo "═══════════════════════════════════════════════════════════════"
echo " M2 TESTING MILESTONE - FINAL VERIFICATION"
echo "═══════════════════════════════════════════════════════════════"
echo ""

echo "=== Running unit tests ==="
go test -short -race ./...

echo ""
echo "=== Checking coverage ==="
go test -coverprofile=coverage.out ./...

echo ""
echo "=== Coverage by package ==="
echo "pkg/tools:"
go test -cover ./pkg/tools/... 2>&1 | grep coverage || echo "No coverage data"
echo ""
echo "pkg/ingestion:"
go test -cover ./pkg/ingestion/... 2>&1 | grep coverage || echo "No coverage data (may require CozoDB library)"
echo ""
echo "pkg/storage:"
go test -cover ./pkg/storage/... 2>&1 | grep coverage || echo "No coverage data"
echo ""
echo "pkg/llm:"
go test -cover ./pkg/llm/... 2>&1 | grep coverage || echo "No coverage data"

echo ""
echo "=== Total coverage ==="
go tool cover -func=coverage.out | grep total

echo ""
echo "=== Running benchmarks ==="
go test -bench=. -benchtime=1s -benchmem ./pkg/tools/... 2>&1 | head -20 || echo "Benchmarks may require CozoDB library"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo " VERIFICATION COMPLETE"
echo "═══════════════════════════════════════════════════════════════"
