#!/bin/bash

# Test 2: System Detection and Platform-Aware Commands

# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing system detection and platform-aware commands..."

# Test system information detection
echo "  Testing system information detection..."
system_info_file=$(create_temp_file)
rag_cli --auto-approve --prompt 'show me the operating system and architecture' >"$system_info_file" 2>&1

if ! grep -q "uname" "$system_info_file" || ! (grep -q "darwin\|linux\|windows" "$system_info_file" || grep -q "arm64\|amd64\|x86_64" "$system_info_file"); then
    echo "FAIL: System detection not working properly"
    echo "Output: $(cat "$system_info_file")"
    exit 1
fi

echo "  PASS: System detection commands used"

# Test platform-specific command execution and result format
echo "  Testing platform-specific command execution..."
platform_test_file=$(create_temp_file)
rag_cli --auto-approve --prompt 'find the largest file in this directory and show its size in bytes' >"$platform_test_file" 2>&1

# Check if a valid command was executed and produced reasonable output
output_content=$(cat "$platform_test_file")

# Verify that some command was executed (look for "Executing:" or "Auto-approving command:")
if ! grep -q "Executing:\|Auto-approving command:" "$platform_test_file"; then
    echo "FAIL: No command execution detected"
    echo "Output: $output_content"
    exit 1
fi

# Verify that the output contains size information (numbers with common size units or just numbers)
if ! echo "$output_content" | grep -qE "[0-9]+[KMGkmg]?B?\s*|[0-9]+\s*(bytes?|size)"; then
    echo "FAIL: No size information found in output"
    echo "Output: $output_content"
    exit 1
fi

# Check that appropriate commands were used for file size detection
# Accept various valid approaches: du, find, stat, ls, etc.
if ! grep -qE "du\s|find\s|stat\s|ls\s" "$platform_test_file"; then
    echo "FAIL: No recognized file system command used"
    echo "Output: $output_content"
    exit 1
fi

echo "  PASS: Platform-appropriate command executed with valid output"

echo "PASS: System detection and platform awareness working"
exit 0
