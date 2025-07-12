#!/bin/bash

# Test 6: File processing, chunking, embeddings, and ChromaDB

echo "Testing file processing, chunking, embeddings, and ChromaDB integration..."

echo "  Creating test files..."
echo "This is a test file about machine learning.
Machine learning is a subset of artificial intelligence.
It focuses on algorithms that can learn from data.
Common techniques include neural networks, decision trees, and support vector machines.
The goal is to make predictions or decisions without being explicitly programmed." >ml_test.txt

echo "This document discusses software engineering principles.
Software engineering involves the systematic approach to designing, developing, and maintaining software.
Key principles include modularity, abstraction, encapsulation, and separation of concerns.
Testing is crucial for ensuring software quality and reliability." >engineering_test.txt

echo "  Testing indexing of multiple files..."
if ! rag_cli index ml_test.txt; then
    echo "FAIL: Could not index ml_test.txt"
    rm -f ml_test.txt engineering_test.txt
    exit 1
fi

if ! rag_cli index engineering_test.txt; then
    echo "FAIL: Could not index engineering_test.txt"
    rm -f ml_test.txt engineering_test.txt
    exit 1
fi

echo "  Testing recursive indexing..."
mkdir -p test_dir
echo "Nested document content about databases.
Databases are structured collections of data.
They can be relational or non-relational.
Common database systems include PostgreSQL, MySQL, and MongoDB." >test_dir/database_info.txt

if ! rag_cli index -r test_dir; then
    echo "FAIL: Could not recursively index test_dir"
    rm -f ml_test.txt engineering_test.txt
    rm -rf test_dir
    exit 1
fi

echo "  Testing different file formats..."
echo '{"name": "test", "description": "JSON test file for RAG CLI"}' >test_config.json
echo "# Markdown Test

This is a **markdown** test file.

- Item 1
- Item 2
- Item 3" >test_readme.md

if ! rag_cli index test_config.json; then
    echo "FAIL: Could not index JSON file"
    rm -f ml_test.txt engineering_test.txt test_config.json test_readme.md
    rm -rf test_dir
    exit 1
fi

if ! rag_cli index test_readme.md; then
    echo "FAIL: Could not index Markdown file"
    rm -f ml_test.txt engineering_test.txt test_config.json test_readme.md
    rm -rf test_dir
    exit 1
fi

echo "  PASS: All file processing tests completed successfully"

# Cleanup
rm -f ml_test.txt engineering_test.txt test_config.json test_readme.md
rm -rf test_dir

echo "PASS: File processing, chunking, embeddings, and ChromaDB integration working"
exit 0
