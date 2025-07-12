#!/bin/bash

# Test 2: System Detection and Platform-Aware Commands

echo "Testing system detection and platform-aware commands..."

# Test system information detection
echo "  Testing system information detection..."
system_info_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'show me the operating system and architecture' >"$system_info_file" 2>&1

if ! grep -q "uname" "$system_info_file" || ! (grep -q "darwin\|linux\|windows" "$system_info_file" || grep -q "arm64\|amd64\|x86_64" "$system_info_file"); then
    echo "FAIL: System detection not working properly"
    echo "Output: $(cat "$system_info_file")"
    exit 1
fi

echo "  PASS: System detection commands used"

# Test platform-specific command syntax
echo "  Testing platform-specific command syntax..."
platform_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'find the largest file in this directory and show its size in bytes' >"$platform_test_file" 2>&1

# Check if correct platform syntax is used based on detected OS
if [[ "$(uname)" == "Darwin" ]]; then
    # On macOS, should use BSD syntax
    if ! grep -q "stat -f" "$platform_test_file"; then
        echo "FAIL: BSD syntax not detected on macOS"
        echo "Output: $(cat "$platform_test_file")"
        exit 1
    fi
    echo "  PASS: Correct BSD syntax used on macOS"
else
    # On Linux, should use GNU syntax
    if ! grep -q "stat -c" "$platform_test_file"; then
        echo "FAIL: GNU syntax not detected on Linux"
        echo "Output: $(cat "$platform_test_file")"
        exit 1
    fi
    echo "  PASS: Correct GNU syntax used on Linux"
fi

echo "PASS: System detection and platform awareness working"
exit 0
