#!/bin/bash

# Test helper functions for RAG CLI integration tests

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where the test script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Create temporary directory for test files if it doesn't exist
if [[ -z "$TEST_TEMP_DIR" ]]; then
    TEST_TEMP_DIR=$(mktemp -d)
    export TEST_TEMP_DIR
    echo "Using temporary directory: $TEST_TEMP_DIR"
    
    # Set up cleanup for individual test runs
    cleanup() {
        echo "Cleaning up temporary files..."
        rm -rf "$TEST_TEMP_DIR"
    }
    trap cleanup EXIT
fi

# Function to create temp files in our test directory
create_temp_file() {
    mktemp "$TEST_TEMP_DIR/test_${RANDOM}_XXXXXX.txt"
}

# Function to run rag-cli using go run from the project root
rag_cli() {
    # Change to project root for execution
    (
        cd "$PROJECT_ROOT"
        # For chat commands, add --no-history to avoid context pollution between tests
        if [[ "$1" == "chat" ]]; then
            go run . "$@" --no-history
        else
            go run . "$@"
        fi
    )
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to print test status
print_status() {
    local status="$1"
    local message="$2"
    
    case "$status" in
        "PASS")
            echo -e "${GREEN}  PASS: $message${NC}"
            ;;
        "FAIL")
            echo -e "${RED}  FAIL: $message${NC}"
            ;;
        "INFO")
            echo -e "${YELLOW}  $message${NC}"
            ;;
        *)
            echo "  $message"
            ;;
    esac
}

# Function to run a test section
run_test_section() {
    local section_name="$1"
    echo "  Testing $section_name..."
}

# Function to check if ChromaDB is running
check_chromadb() {
    if ! curl -s http://localhost:8000/api/v1/heartbeat >/dev/null 2>&1; then
        echo -e "${RED}ERROR: ChromaDB is not running on localhost:8000${NC}"
        echo "Please start ChromaDB with: docker run -p 8000:8000 chromadb/chroma"
        exit 1
    fi
}

# Function to wait for file creation with timeout
wait_for_file() {
    local file_path="$1"
    local timeout="${2:-10}"  # Default 10 seconds
    local elapsed=0
    
    while [[ ! -f "$file_path" && $elapsed -lt $timeout ]]; do
        sleep 0.5
        elapsed=$((elapsed + 1))
    done
    
    [[ -f "$file_path" ]]
}

# Function to clean up test files
cleanup_test_files() {
    local pattern="$1"
    if [[ -n "$pattern" ]]; then
        rm -f $pattern 2>/dev/null || true
    fi
}

# Function to get unique collection names for testing
get_test_collections() {
    local timestamp=$(date +%s)
    echo "test_docs_$timestamp test_commands_$timestamp test_auto_$timestamp"
}

# Function to check if string contains expected content
contains() {
    local haystack="$1"
    local needle="$2"
    [[ "$haystack" == *"$needle"* ]]
}

# Function to count lines in output
count_lines() {
    local content="$1"
    echo "$content" | wc -l | tr -d ' '
}

# Function to extract numeric values from text
extract_number() {
    local text="$1"
    echo "$text" | grep -oE '[0-9]+' | head -1
}

# Function to check if test should be skipped based on platform
should_skip_test() {
    local test_name="$1"
    local reason="$2"
    
    if [[ -n "$reason" ]]; then
        print_status "INFO" "Skipping $test_name: $reason"
        return 0
    fi
    return 1
}

# Function to get the current platform
get_platform() {
    case "$(uname -s)" in
        Darwin*)    echo "macos" ;;
        Linux*)     echo "linux" ;;
        MINGW*)     echo "windows" ;;
        *)          echo "unknown" ;;
    esac
}

# Export functions so they can be used in test scripts
export -f create_temp_file
export -f rag_cli
export -f command_exists
export -f print_status
export -f run_test_section
export -f check_chromadb
export -f wait_for_file
export -f cleanup_test_files
export -f get_test_collections
export -f contains
export -f count_lines
export -f extract_number
export -f should_skip_test
export -f get_platform
