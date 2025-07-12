#!/bin/bash

# Test 1: Basic Chat Command Execution

# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing basic chat command capability..."

# Use temp file for output
output_file=$(create_temp_file)

# Test basic file creation command
rag_cli chat --auto-approve --prompt 'create a file called "hello.txt" with the contents "written by rag-cli"' > "$output_file" 2>&1

if [[ ! -f hello.txt ]]; then
    echo "FAIL: File not created"
    echo "Output: $(cat "$output_file")"
    exit 1
fi

contents=$(cat hello.txt)
if [[ "$contents" != "written by rag-cli" ]]; then
    echo "FAIL: Expected 'written by rag-cli' but got '$contents'"
    rm -f hello.txt
    exit 1
fi

echo "PASS: Basic command execution successful"
rm -f hello.txt
exit 0
