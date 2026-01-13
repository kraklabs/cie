#!/bin/bash
# test-quick-start.sh - Automated CIE quick start validation
# Part of EPIC-001 M3 T060 - Test Quick Start from Scratch
#
# This script validates that the CIE quick start works as documented in README.md.
# It measures timing for each step and verifies commands work correctly.
#
# Usage:
#   bash test-quick-start.sh
#
# Exit codes:
#   0 - Success (all steps passed)
#   1 - Failure (one or more steps failed)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test project directory
TEST_DIR="/tmp/cie-test-$(date +%s)"

echo "═════════════════════════════════════════════════════════════"
echo "  CIE QUICK START TEST"
echo "═════════════════════════════════════════════════════════════"
echo ""
echo "Platform: $(uname -s) $(uname -r)"
echo "Date: $(date)"
echo "Test Directory: $TEST_DIR"
echo ""

START_TIME=$(date +%s)

# Track failures
FAILURES=0

# Helper function for timing
time_step() {
    local step_name="$1"
    local step_num="$2"
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Step $step_num: $step_name${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Step 0: Prerequisites check
time_step "Prerequisites Check" "0"

echo "Checking required tools..."

# Check Go
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓${NC} Go: $GO_VERSION"
else
    echo -e "${RED}✗${NC} Go not found (required)"
    FAILURES=$((FAILURES + 1))
fi

# Check CIE binary exists (from earlier build)
if command -v cie &> /dev/null; then
    CIE_PATH=$(which cie)
    echo -e "${GREEN}✓${NC} CIE: $CIE_PATH"
else
    echo -e "${YELLOW}⚠${NC} CIE not in PATH (will try from modules/cie)"
fi

# Check CozoDB library
if [ -f "/usr/local/lib/libcozo_c.so" ] || [ -f "/usr/local/lib/libcozo_c.dylib" ]; then
    echo -e "${GREEN}✓${NC} CozoDB C library found"
else
    echo -e "${YELLOW}⚠${NC} CozoDB C library not found (may cause build failures)"
fi

# Check Ollama (optional but recommended)
if command -v ollama &> /dev/null; then
    echo -e "${GREEN}✓${NC} Ollama: $(ollama --version 2>&1 | head -1)"
else
    echo -e "${YELLOW}⚠${NC} Ollama not found (optional, but recommended for embeddings)"
fi

# Step 1: Create test project
time_step "Create Test Project" "1"
STEP1_START=$(date +%s)

mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

# Create a simple Go project to index
cat > main.go <<'EOF'
package main

import "fmt"

// HelloWorld prints a greeting message
func HelloWorld() {
    fmt.Println("Hello, CIE!")
}

// Add adds two integers
func Add(a, b int) int {
    return a + b
}

func main() {
    HelloWorld()
    result := Add(2, 3)
    fmt.Printf("2 + 3 = %d\n", result)
}
EOF

# Create go.mod
cat > go.mod <<EOF
module example.com/test

go 1.24
EOF

STEP1_END=$(date +%s)
STEP1_DURATION=$((STEP1_END - STEP1_START))
echo -e "${GREEN}✓${NC} Test project created in ${STEP1_DURATION}s"

# Step 2: Initialize CIE
time_step "Initialize CIE (cie init)" "2"
STEP2_START=$(date +%s)

# Run cie init from the monorepo if binary not in PATH
if command -v cie &> /dev/null; then
    cie init
else
    # Assume we're running from kraken root
    go run /Users/francisco/code/kraklabs/kraken/modules/cie/cmd/cie init
fi

STEP2_END=$(date +%s)
STEP2_DURATION=$((STEP2_END - STEP2_START))

# Verify .cie directory was created
if [ -d ".cie" ] && [ -f ".cie/project.yaml" ]; then
    echo -e "${GREEN}✓${NC} CIE initialized successfully in ${STEP2_DURATION}s"
    echo "   Created: .cie/project.yaml"
else
    echo -e "${RED}✗${NC} CIE initialization failed (no .cie/project.yaml found)"
    FAILURES=$((FAILURES + 1))
fi

# Step 3: Index the repository
time_step "Index Repository (cie index)" "3"
STEP3_START=$(date +%s)

if command -v cie &> /dev/null; then
    cie index
else
    go run /Users/francisco/code/kraklabs/kraken/modules/cie/cmd/cie index
fi

STEP3_END=$(date +%s)
STEP3_DURATION=$((STEP3_END - STEP3_START))

echo -e "${GREEN}✓${NC} Indexing completed in ${STEP3_DURATION}s"

# Step 4: Check status
time_step "Check Status (cie status)" "4"
STEP4_START=$(date +%s)

if command -v cie &> /dev/null; then
    cie status
else
    go run /Users/francisco/code/kraklabs/kraken/modules/cie/cmd/cie status
fi

STEP4_END=$(date +%s)
STEP4_DURATION=$((STEP4_END - STEP4_START))

echo -e "${GREEN}✓${NC} Status command successful in ${STEP4_DURATION}s"

# Calculate total time
END_TIME=$(date +%s)
TOTAL_DURATION=$((END_TIME - START_TIME))

# Print summary
echo ""
echo "═════════════════════════════════════════════════════════════"
echo "  TEST RESULTS"
echo "═════════════════════════════════════════════════════════════"
echo ""
echo "Timing Breakdown:"
echo "  Step 1 (Create project): ${STEP1_DURATION}s"
echo "  Step 2 (Initialize):     ${STEP2_DURATION}s"
echo "  Step 3 (Index):          ${STEP3_DURATION}s"
echo "  Step 4 (Status):         ${STEP4_DURATION}s"
echo "  ─────────────────────────────────"
echo "  Total:                   ${TOTAL_DURATION}s"
echo ""

# Evaluate results
if [ $FAILURES -eq 0 ]; then
    if [ $TOTAL_DURATION -lt 300 ]; then
        echo -e "${GREEN}✓ SUCCESS: All steps passed in under 5 minutes!${NC}"
        exit 0
    else
        echo -e "${YELLOW}⚠ SLOW: All steps passed but took more than 5 minutes${NC}"
        echo "  (${TOTAL_DURATION}s total, target was 300s)"
        exit 0
    fi
else
    echo -e "${RED}✗ FAILED: $FAILURES step(s) failed${NC}"
    exit 1
fi

# Cleanup (optional - comment out to inspect results)
# cd /
# rm -rf "$TEST_DIR"
# echo "Test directory cleaned up"
