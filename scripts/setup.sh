#!/bin/bash

set -e

# Install dependencies
echo "Installing dependencies..."
brew install ollama

# Pull models
echo "Pulling models..."
ollama pull granite-code:3b
ollama pull all-minilm

# Build the project
echo "Building the project..."
go build -o rag-cli

# Start Docker
cd docker
echo "Starting Docker services..."
docker-compose up -d

cd ..

# Setup complete
echo "Setup complete! Run './rag-cli --help' to get started."
