#!/bin/bash

# Test 7: Command Evaluation and Goal Recognition


# Get the directory where this script is located and source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

echo "Testing command evaluation and goal recognition..."

# Test simple goal recognition
echo "  Testing simple goal recognition (should complete in 1 attempt)..."
goal_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'show me the current working directory path' >"$goal_test_file" 2>&1

# Check if it completed successfully without multiple attempts
if grep -q "Attempt [2-3]" "$goal_test_file"; then
    echo "FAIL: Simple goal should complete in one attempt"
    echo "Output: $(head -10 "$goal_test_file")"
    exit 1
fi

# Check if it shows a directory path (should contain the project path)
if ! grep -q "/rag-cli" "$goal_test_file"; then
    echo "FAIL: Should show current directory path"
    echo "Output: $(head -10 "$goal_test_file")"
    exit 1
fi

echo "  PASS: Simple goal recognition - single attempt"

# Test command filtering
echo "  Testing command filtering (should not show errors)..."
filtering_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'count the number of files in the current directory' >"$filtering_test_file" 2>&1

# Check if command executed successfully (should show a number)
if grep -q "Error: command failed: exit status 127" "$filtering_test_file"; then
    echo "FAIL: Command execution failed with errors"
    echo "Output: $(head -10 "$filtering_test_file")"
    exit 1
fi

# Should contain some numeric output
if ! grep -qE "[0-9]+" "$filtering_test_file"; then
    echo "FAIL: Should show file count as a number"
    echo "Output: $(head -10 "$filtering_test_file")"
    exit 1
fi

echo "  PASS: Command filtering - executed successfully"

# Test structured evaluation
echo "  Testing structured evaluation (should show goal achievement)..."
evaluation_test_file=$(create_temp_file)
rag_cli chat --allow-commands --auto-approve --prompt 'display the current date' >"$evaluation_test_file" 2>&1

# Check if evaluation is working properly
if ! grep -q "date" "$evaluation_test_file" || grep -q "Max attempts" "$evaluation_test_file"; then
    echo "FAIL: Structured evaluation not working properly"
    echo "Output: $(head -10 "$evaluation_test_file")"
    exit 1
fi

echo "  PASS: Structured evaluation - goal achieved without max attempts"
echo "PASS: Command evaluation and goal recognition working"
exit 0
