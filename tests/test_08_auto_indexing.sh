#!/bin/bash

# Test 8: Auto-Indexing of File Changes


# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing auto-indexing of file changes..."

# Test file creation auto-indexing
echo "  Testing file creation auto-indexing..."
auto_index_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --auto-index --prompt 'create a file called auto_test.py with content "# Auto-index test file"' >"$auto_index_test_file" 2>&1

# Check if auto-indexing was triggered
if ! grep -q "\[Auto-indexing" "$auto_index_test_file" || [[ ! -f auto_test.py ]]; then
    echo "FAIL: File creation auto-indexing not working"
    echo "Output: $(head -5 "$auto_index_test_file")"
    rm -f auto_test.py
    exit 1
fi

echo "  PASS: File creation triggers auto-indexing"
rm -f auto_test.py

# Test file filtering (should not index log files)
echo "  Testing file filtering (should not index log files)..."
filter_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --auto-index --prompt 'create a log file called test.log' >"$filter_test_file" 2>&1

# Check that .log files are not auto-indexed
if grep "\[Auto-indexing" "$filter_test_file" | grep -q "test.log" || [[ ! -f test.log ]]; then
    echo "FAIL: File filtering not working properly"
    echo "Output: $(head -5 "$filter_test_file")"
    rm -f test.log
    exit 1
fi

echo "  PASS: File filtering - log files not indexed"
rm -f test.log

# Test multiple file creation
echo "  Testing multiple file creation..."
multi_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --auto-index --prompt 'create files: test.txt with "hello" and test.py with "print(1)"' >"$multi_test_file" 2>&1

# Check if multiple files trigger auto-indexing
if ! (grep -q "test.txt" "$multi_test_file" || grep -q "test.py" "$multi_test_file") || [[ ! -f test.txt ]] || [[ ! -f test.py ]]; then
    echo "FAIL: Multiple file auto-indexing not working"
    echo "Output: $(head -5 "$multi_test_file")"
    rm -f test.txt test.py
    exit 1
fi

echo "  PASS: Multiple file auto-indexing working"
rm -f test.txt test.py

echo "PASS: Auto-indexing of file changes working"
exit 0
