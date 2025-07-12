#!/bin/bash

# Test 4: Multi-step Command Execution with Learning


# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing multi-step execution with learning..."

echo "  Testing iterative command execution..."
multistep_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'first run: mkdir test-workspace, then run: echo "test file" > test-workspace/test.txt' >"$multistep_test_file" 2>&1

if [[ ! -d test-workspace ]] || [[ ! -f test-workspace/test.txt ]]; then
    echo "FAIL: Multi-step execution did not complete successfully"
    echo "Output: $(head -20 "$multistep_test_file")"
    exit 1
fi

echo "  PASS: Multi-step directory and file creation successful"
rm -rf test-workspace
echo "PASS: Multi-step execution with learning working"
exit 0
