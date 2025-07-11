#!/bin/bash

set -e

echo "Pulling required models for Ollama..."

# Pull the language model
echo "Pulling granite-code:3b..."
docker exec ollama-server ollama pull granite-code:3b

# Pull the embedding model
echo "Pulling all-minilm..."
docker exec ollama-server ollama pull all-minilm

echo "Models pulled successfully!"
echo "Available models:"
docker exec ollama-server ollama list
