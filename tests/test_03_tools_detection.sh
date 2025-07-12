#!/bin/bash

# Test 3: Available Tools Detection


# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing available tools detection..."

echo "  Testing tool availability awareness..."
tools_test_file=$(create_temp_file)
rag_cli chat --auto-approve --prompt 'what version of git is installed?' >"$tools_test_file" 2>&1

if ! grep -q "git version" "$tools_test_file"; then
    echo "FAIL: Git detection not working"
    echo "Output: $(head -20 "$tools_test_file")"
    exit 1
fi

echo "  PASS: Git detection and version check working"
echo "PASS: Available tools detection working"
exit 0
