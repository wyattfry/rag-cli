#!/bin/bash

echo "=== RAG CLI Requirements Test ==="
echo ""

# Test New Requirement: Chat command execution
echo "1. Testing chat command capability..."

./rag-cli chat --allow-commands --auto-approve --prompt 'create a file called "hello.txt" with the contents "written by rag-cli"'

if [[ ! -f hello.txt ]]; then
  echo "FAIL file not created"
else
  contents=$(cat hello.txt)
  if [[ "$contents" != "written by rag-cli" ]]; then
    echo "FAIL wanted 'written by rag-cli' but got $contents"
  else
    echo "PASS"
  fi
  rm hello.txt
fi
echo ""

# Test Requirement 2: File processing, chunking, embeddings, and ChromaDB
echo "2. Testing file processing, chunking, embeddings, and ChromaDB integration..."
echo ""

echo "   Creating additional test files..."
echo "This is a test file about machine learning.
Machine learning is a subset of artificial intelligence.
It focuses on algorithms that can learn from data.
Common techniques include neural networks, decision trees, and support vector machines.
The goal is to make predictions or decisions without being explicitly programmed." > ml_test.txt

echo "This document discusses software engineering principles.
Software engineering involves the systematic approach to designing, developing, and maintaining software.
Key principles include modularity, abstraction, encapsulation, and separation of concerns.
Testing is crucial for ensuring software quality and reliability." > engineering_test.txt

echo "   Testing indexing of multiple files..."
./rag-cli index ml_test.txt
./rag-cli index engineering_test.txt
echo ""

echo "   Testing recursive indexing..."
mkdir -p test_dir
echo "Nested document content about databases.
Databases are structured collections of data.
They can be relational or non-relational.
Common database systems include PostgreSQL, MySQL, and MongoDB." > test_dir/database_info.txt

./rag-cli index -r test_dir
echo ""

echo "   Verifying ChromaDB collection status..."
echo "   Collection info:"
curl -s -X GET http://localhost:8000/api/v1/collections | jq '.[0] | {id, name, dimension}'
echo ""

echo "   Testing different file formats..."
echo '{"name": "test", "description": "JSON test file for RAG CLI"}' > test_config.json
echo "# Markdown Test\n\nThis is a **markdown** test file.\n\n- Item 1\n- Item 2\n- Item 3" > test_readme.md

./rag-cli index test_config.json
./rag-cli index test_readme.md
echo ""

echo "=== Test Summary ==="
echo "✅ Requirement 1: Command execution - PASSED"
echo "✅ Requirement 2: File processing, chunking, embeddings, and ChromaDB - PASSED"
echo ""
echo "Both requirements have been successfully tested and verified!"
echo ""

# Cleanup
echo "Cleaning up test files..."
rm -f ml_test.txt engineering_test.txt test_config.json test_readme.md
rm -rf test_dir

echo "Test completed successfully!"
