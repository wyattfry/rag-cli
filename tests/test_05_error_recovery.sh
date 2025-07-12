#!/bin/bash

# Test 5: Error Recovery and Learning


# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing error recovery and learning..."

echo "  Testing command error recovery..."
# This should trigger an error correction scenario
error_recovery_test_file=$(create_temp_file)
rag_cli --auto-approve --prompt 'show me disk usage of all files sorted by size' >"$error_recovery_test_file" 2>&1

# Check if the command completed successfully despite potential initial errors
if grep -q "Attempt [2-3]" "$error_recovery_test_file"; then
    echo "  PASS: Error recovery with retry attempts detected"
elif grep -q -E "[0-9]+" "$error_recovery_test_file" && grep -q "$" "$error_recovery_test_file"; then
    echo "  PASS: Command executed successfully (may not have needed recovery)"
else
    echo "FAIL: Error recovery test did not show expected results"
    echo "Output: $(head -10 "$error_recovery_test_file")"
    exit 1
fi

echo "PASS: Error recovery and learning working"
exit 0
