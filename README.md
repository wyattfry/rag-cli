# RAG CLI

A command-line tool with RAG (Retrieval-Augmented Generation) capabilities using local models.

## Features

- **Local Models**: Uses locally running language models (e.g., Ollama with Granite)
- **Vector Database**: Integrates with Chroma for storing and retrieving embeddings
- **Document Indexing**: Automatically chunks and indexes documents
- **Interactive Chat**: AI-powered chat with contextual responses
- **Command Execution**: Execute shell commands through the CLI
- **Multiple File Formats**: Supports various file types (txt, md, go, py, js, ts, json, yaml, etc.)

## Prerequisites

1. **Docker**: Required for running services
2. **Go**: For building the CLI tool

## Setup

You can run RAG CLI in two ways:

### Option 1: Native Ollama + Docker ChromaDB (Recommended for macOS)

This approach uses native Ollama installation with ChromaDB in Docker. **This is the recommended approach for macOS users** as Docker Desktop on macOS does not support GPU passthrough, meaning containerized Ollama cannot access your Apple Silicon GPU for acceleration.

#### Why Native Ollama on macOS?
- **GPU Acceleration**: Native Ollama can utilize Apple Silicon's Metal Performance Shaders (MPS) for significantly faster inference
- **Better Performance**: Direct access to system resources without Docker overhead
- **Simpler Setup**: No need for complex Docker GPU configurations that don't work on macOS anyway

#### Quick Start
```bash
# Clone and build
git clone <repository-url>
cd rag-cli
go build -o rag-cli

# Install Ollama natively
brew install ollama

# Start ChromaDB in Docker
cd docker
docker-compose -f docker-compose-chroma-only.yaml up -d

# Start Ollama in the background
ollama serve &

# Pull required models
ollama pull granite-code:3b
ollama pull all-minilm

# You're ready to go!
./rag-cli --help
```

#### Manual Setup
1. **Install Ollama Locally**
   ```bash
   # On macOS
   brew install ollama
   
   # Or download from https://ollama.ai
   ```

2. **Start ChromaDB in Docker**
   ```bash
   cd docker
   docker-compose -f docker-compose-chroma-only.yaml up -d
   ```

3. **Start Ollama Server**
   ```bash
   ollama serve
   ```

4. **Pull Required Models**
   ```bash
   # Pull language model
   ollama pull granite-code:3b
   
   # Pull embedding model
   ollama pull all-minilm
   ```

5. **Build CLI**
   ```bash
   go build -o rag-cli
   ```

### Option 2: Fully Dockerized (Linux/Windows)

This approach runs both Ollama and ChromaDB in Docker containers. **Note: This is not recommended for macOS due to lack of GPU passthrough support.**

#### For Linux with NVIDIA GPU:
```bash
# Use the NVIDIA-specific compose file
cd docker
docker-compose -f docker-compose-linux-nvidia.yml up -d
```

#### For other platforms (CPU-only):
```bash
# Use the basic compose file
cd docker
# Note: No basic docker-compose.yml exists yet - you'd need to create one
```

### Using the Makefile

For easier setup, you can use the provided Makefile:

```bash
# Full setup (Docker approach)
make setup

# Individual commands
make docker-up    # Start Docker services
make models       # Pull models
make build        # Build CLI
make clean        # Clean build artifacts
make docker-down  # Stop Docker services
```

## Configuration

The CLI uses default settings that work with both Docker and native deployments. You can optionally create a config file at `~/.rag-cli.yaml` to customize settings:

```yaml
llm:
  model: "granite-code:3b"
  host: "localhost"
  port: 11434
  base_url: "http://localhost:11434"

vector:
  host: "localhost"
  port: 8000
  collection: "documents"

embeddings:
  model: "all-minilm"
  host: "localhost"
  port: 11434
  base_url: "http://localhost:11434"

chunker:
  chunk_size: 1000
  chunk_overlap: 200
```

**Note**: The default configuration works for both Docker and native setups since both use the same ports (11434 for Ollama, 8000 for ChromaDB).

### Alternative Models

You can use different models by updating the config:

```yaml
llm:
  model: "llama3.1:8b"  # or "mistral:7b", "codellama:7b", etc.
  
embeddings:
  model: "nomic-embed-text"  # or "all-minilm", "sentence-transformers", etc.
```

Make sure to pull the models first:
```bash
# For Docker setup
docker exec ollama-server ollama pull llama3.1:8b

# For native setup
ollama pull llama3.1:8b
```

## Usage

### Index Documents
```bash
# Index current directory
./rag-cli index

# Index specific directory recursively
./rag-cli index -r /path/to/docs

# Index with specific file formats
./rag-cli index -f txt,md,go /path/to/project
```

### Start Interactive Chat
```bash
./rag-cli chat
```

### Execute Commands
```bash
./rag-cli exec "ls -la"
./rag-cli exec "git status"
```

### Get Help
```bash
./rag-cli --help
./rag-cli chat --help
./rag-cli index --help
```

## Architecture

- **CLI Layer**: Cobra-based command-line interface
- **LLM Integration**: HTTP client for Ollama API
- **Vector Store**: Chroma database for embeddings
- **Chunking**: Text splitting for optimal embedding generation
- **Embeddings**: Local embedding generation via Ollama

## Development

### Project Structure
```
rag-cli/
├── cmd/           # CLI commands
├── internal/      # Internal packages
│   ├── llm/       # Language model client
│   ├── vector/    # Vector database client
│   ├── embeddings/# Embedding generation
│   └── chunker/   # Text chunking
├── pkg/           # Public packages
│   ├── config/    # Configuration management
│   └── models/    # Data models
├── docker/        # Docker configurations
└── scripts/       # Utility scripts
```

### Adding New Commands
1. Create a new file in `cmd/` directory
2. Implement the command using Cobra
3. Add it to the root command in `init()`

### Extending Functionality
- Add new vector database backends in `internal/vector/`
- Support additional LLM providers in `internal/llm/`
- Implement advanced chunking strategies in `internal/chunker/`

## Troubleshooting

### Common Issues

#### Docker Setup Issues
1. **Services not starting**: Check Docker is running and ports are available
   ```bash
   docker ps  # Check running containers
   docker-compose logs  # Check service logs
   ```

2. **Models not found in Docker**: Pull models into the container
   ```bash
   docker exec ollama-server ollama list  # Check available models
   ./scripts/pull-models.sh  # Pull required models
   ```

3. **ChromaDB connection failed**: Ensure container is running
   ```bash
   curl -X GET http://localhost:8000/api/v1/heartbeat
   ```

#### Native Setup Issues
1. **Ollama not responding**: Make sure Ollama is running
   ```bash
   ollama serve  # Start Ollama server
   curl -X GET http://localhost:11434/api/tags  # Test connection
   ```

2. **Models not found**: Pull required models
   ```bash
   ollama pull granite-code:3b
   ollama pull all-minilm
   ```

#### General Issues
1. **Permission errors**: Check file permissions for indexing
2. **Port conflicts**: Ensure ports 8000 and 11434 are available
3. **Memory issues**: Large models require significant RAM

### Debug Mode
```bash
./rag-cli --debug chat
```

### Checking Service Status
```bash
# Check all services
make docker-up
docker ps

# Test Ollama
curl -X GET http://localhost:11434/api/tags

# Test ChromaDB
curl -X GET http://localhost:8000/api/v1/heartbeat
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
