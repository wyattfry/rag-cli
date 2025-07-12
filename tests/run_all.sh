#!/bin/bash

# Test runner for RAG CLI
# Runs all test files matching the pattern test_*.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Create temporary directory for all test files
TEST_TEMP_DIR=$(mktemp -d)
export TEST_TEMP_DIR
echo "Using temporary directory: $TEST_TEMP_DIR"

# Function to create temp files in our test directory
create_temp_file() {
    mktemp "$TEST_TEMP_DIR/test_${RANDOM}_XXXXXX.txt"
}
export -f create_temp_file

# Function to run rag-cli using go run
rag_cli() {
    # For chat commands, add --no-history to avoid context pollution between tests
    if [[ "$1" == "chat" ]]; then
        go run . "$@" --no-history
    else
        go run . "$@"
    fi
}
export -f rag_cli

# Cleanup function
cleanup() {
    echo "Cleaning up temporary files..."
    rm -rf "$TEST_TEMP_DIR"
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Change to project root
cd "$PROJECT_ROOT"

echo "=== RAG CLI Test Suite ==="
echo "Running from: $PROJECT_ROOT"
echo ""

# Initialize counters
total_tests=0
passed_tests=0
failed_tests=0

# Array to store test results
declare -a test_results

# Find and run all test files
for test_file in "$SCRIPT_DIR"/test_*.sh; do
    if [[ -f "$test_file" ]]; then
        test_name=$(basename "$test_file" .sh)
        echo -e "${YELLOW}Running $test_name...${NC}"
        
        total_tests=$((total_tests + 1))
        
        # Run the test and capture exit code
        if bash "$test_file"; then
            echo -e "${GREEN}✅ $test_name PASSED${NC}"
            passed_tests=$((passed_tests + 1))
            test_results+=("$test_name:PASS")
        else
            echo -e "${RED}❌ $test_name FAILED${NC}"
            failed_tests=$((failed_tests + 1))
            test_results+=("$test_name:FAIL")
        fi
        
        echo ""
    fi
done

# Summary
echo "=== Test Summary ==="
echo "Total tests: $total_tests"
echo -e "Passed: ${GREEN}$passed_tests${NC}"
echo -e "Failed: ${RED}$failed_tests${NC}"
echo ""

# Detailed results
echo "=== Detailed Results ==="
for result in "${test_results[@]}"; do
    test_name="${result%:*}"
    status="${result#*:}"
    if [[ "$status" == "PASS" ]]; then
        echo -e "${GREEN}✅ $test_name${NC}"
    else
        echo -e "${RED}❌ $test_name${NC}"
    fi
done

echo ""
if [[ $failed_tests -eq 0 ]]; then
    echo -e "${GREEN}All tests passed! 🎉${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please check the output above.${NC}"
    exit 1
fi
