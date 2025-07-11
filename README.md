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

1. **Docker**: Required for running Chroma vector database
2. **Ollama**: For running local language models
3. **Go**: For building the CLI tool

## Setup

### 1. Install Ollama
```bash
# On macOS
brew install ollama

# Or download from https://ollama.ai
```

### 2. Pull Required Models
```bash
# Pull a language model (e.g., Granite Code)
ollama pull granite-code:3b

# Pull an embedding model
ollama pull all-minilm
```

### 3. Start Ollama Server
```bash
ollama serve
```

### 4. Start Chroma Database
```bash
cd docker
docker-compose up -d
```

### 5. Build the CLI Tool
```bash
go build -o rag-cli
```

## Configuration

Create a config file at `~/.rag-cli.yaml`:

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

1. **Ollama not responding**: Make sure Ollama is running (`ollama serve`)
2. **Chroma connection failed**: Check if Docker container is running
3. **Models not found**: Pull required models using `ollama pull`
4. **Permission errors**: Check file permissions for indexing

### Debug Mode
```bash
./rag-cli --debug chat
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
