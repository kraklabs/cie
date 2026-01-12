#!/bin/bash
# Script to run integration tests with CozoDB
# Requires libcozo_c.dylib to be installed in /usr/local/lib

set -e

# Set library path for CozoDB
export DYLD_LIBRARY_PATH=/usr/local/lib:$DYLD_LIBRARY_PATH
export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH

# Run integration tests with coverage
echo "Running integration tests with CozoDB..."
go test -tags=cozodb -coverprofile=coverage_integration.out ./pkg/tools/...

# Show coverage summary
echo ""
echo "Coverage summary:"
go tool cover -func=coverage_integration.out | tail -1

echo ""
echo "To view detailed coverage report:"
echo "  go tool cover -html=coverage_integration.out"
