# RAG CLI

A command-line tool with RAG (Retrieval-Augmented Generation) capabilities using local models. The AI assistant can execute shell commands, learn from past interactions, and provide contextual responses based on your indexed documents.

## Features

- **Local Models**: Uses locally running language models (e.g., Ollama with Granite)
- **Vector Database**: Integrates with Chroma for storing and retrieving embeddings
- **Document Indexing**: Automatically chunks and indexes documents
- **Interactive Chat**: AI-powered chat with contextual responses
- **Intelligent Command Execution**: AI can execute shell commands with iterative feedback and error correction
- **Learning from Experience**: AI learns from past command executions and improves over time
- **Platform-Aware**: Automatically handles macOS vs Linux command syntax differences
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

### Recommended Models

For optimal command execution and learning capabilities, consider these models:

#### Language Models
- **Google Gemma 2 9B SimPO**: Excellent instruction following for command generation
- **Meta Llama 3.1/3.2**: Great reasoning abilities for complex multi-step tasks
- **Microsoft Phi 3.5**: Compact but capable, good for resource-constrained environments
- **Qwen/CodeQwen 2.5 14B**: Specifically designed for coding and command-line tasks
- **granite-code:3b**: Default model, good balance of speed and capability

#### Embedding Models
- **all-minilm**: Default, good general-purpose embedding model
- **nomic-embed-text**: Better for technical documentation and code
- **sentence-transformers**: Good for semantic similarity tasks

### Alternative Models

You can use different models by updating the config:

```yaml
llm:
  model: "gemma2:9b"  # or "llama3.1:8b", "phi3.5", "qwen2.5-coder:14b", etc.
  
embeddings:
  model: "nomic-embed-text"  # or "all-minilm", "sentence-transformers", etc.
```

Make sure to pull the models first:
```bash
# For Docker setup
docker exec ollama-server ollama pull gemma2:9b

# For native setup
ollama pull gemma2:9b
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

### Interactive Chat
```bash
# Start interactive chat
./rag-cli chat

# Enable command execution (with user approval)
./rag-cli chat --allow-commands

# Single prompt with command execution
./rag-cli chat --allow-commands --prompt "create a backup of my config files"

# Auto-approve commands (use with caution)
./rag-cli chat --allow-commands --auto-approve --prompt "show me the largest files"
```

### Example Interactions

#### Basic File Operations
```bash
./rag-cli chat --allow-commands --prompt "create a file called notes.txt with today's date"
```

**AI Response:**
```
The AI wants to execute the following command(s):
echo "$(date)" > notes.txt
Do you want to allow this? (y/n): y

Executing: echo "$(date)" > notes.txt
$ echo "$(date)" > notes.txt

File created successfully!
```

#### System Information
```bash
./rag-cli chat --allow-commands --prompt "what's the current system load and memory usage?"
```

**AI Response:**
```
The AI wants to execute the following command(s):
uptime
free -h
Do you want to allow this? (y/n): y

Executing: uptime
$ uptime
 15:30:45 up 2 days,  8:15,  3 users,  load average: 0.52, 0.58, 0.59

AI suggests next command(s): [vm_stat | head -10]

Attempt 2/3

Executing: vm_stat | head -10
$ vm_stat | head -10
Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               45123.
Pages active:                            234567.
...
```

#### Learning from Mistakes
```bash
./rag-cli chat --allow-commands --prompt "find all Python files and count lines of code"
```

**First attempt (shows error correction):**
```
Executing: find . -name "*.py" -exec wc -l {} \;
$ find . -name "*.py" -exec wc -l {} \;
Command execution failed: no Python files found

AI suggests next command(s): [find . -name "*.py" | wc -l]

Attempt 2/3

Executing: find . -name "*.py" | wc -l
$ find . -name "*.py" | wc -l
0

No Python files found in current directory.
```

#### Complex Multi-Step Tasks
```bash
./rag-cli chat --allow-commands --prompt "create a project structure for a Go web API"
```

**AI Response:**
```
The AI wants to execute the following command(s):
mkdir -p myapi/{cmd,internal/{handlers,models,db},pkg,docs}
touch myapi/main.go myapi/go.mod myapi/README.md
Do you want to allow this? (y/n): y

Executing: mkdir -p myapi/{cmd,internal/{handlers,models,db},pkg,docs}
$ mkdir -p myapi/{cmd,internal/{handlers,models,db},pkg,docs}

AI suggests next command(s): [touch myapi/main.go myapi/go.mod myapi/README.md]

Attempt 2/3

Executing: touch myapi/main.go myapi/go.mod myapi/README.md
$ touch myapi/main.go myapi/go.mod myapi/README.md

Project structure created successfully!
```

### Get Help
```bash
./rag-cli --help
./rag-cli chat --help
./rag-cli index --help
```

## Architecture

- **CLI Layer**: Cobra-based command-line interface
- **LLM Integration**: HTTP client for Ollama API with intelligent prompting
- **Vector Store**: Chroma database for embeddings and execution history
- **Chunking**: Text splitting for optimal embedding generation
- **Embeddings**: Local embedding generation via Ollama
- **Command Execution**: Iterative command execution with feedback loops
- **Learning System**: Historical command execution storage and retrieval
- **Platform Detection**: Automatic macOS/Linux command syntax handling

## How It Works

### Command Execution Flow
1. **User Query**: User asks for a task (e.g., "find the largest files")
2. **Historical Context**: AI retrieves similar past command executions from ChromaDB
3. **Initial Commands**: AI generates appropriate shell commands using platform-aware syntax
4. **Iterative Execution**: Commands are executed one at a time
5. **Feedback Loop**: After each command, AI evaluates results and suggests next steps
6. **Error Correction**: Failed commands trigger AI to suggest corrected alternatives
7. **Session Storage**: Complete execution logs are stored in ChromaDB for future learning

### Learning and Improvement
- **Syntax Learning**: AI learns platform-specific command syntax (macOS vs Linux)
- **Pattern Recognition**: Similar queries benefit from past successful approaches
- **Error Avoidance**: Common mistakes are learned and avoided in future executions
- **Continuous Improvement**: The system gets better over time with more usage

### Safety Features
- **User Approval**: Commands require explicit user approval (unless `--auto-approve` is used)
- **Attempt Limits**: Maximum 3 attempts per command sequence to prevent infinite loops
- **Command Preview**: Shows all commands before execution
- **Execution Logging**: Full command history with inputs, outputs, and errors

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

# Test command execution
./rag-cli chat --allow-commands --auto-approve --prompt "echo 'test'"
```

### Command Execution Issues

#### Commands Not Executing
1. **Missing --allow-commands flag**: Command execution is disabled by default
   ```bash
   ./rag-cli chat --allow-commands  # Enable command execution
   ```

2. **Platform syntax errors**: AI might use wrong command syntax
   - The AI learns from these errors and improves over time
   - Historical execution data helps avoid repeated mistakes

3. **Permission issues**: Commands might fail due to insufficient permissions
   ```bash
   # Check if the command works manually
   ls -la  # Test basic command
   ```

#### AI Not Learning from Mistakes
1. **ChromaDB storage issues**: Check if execution sessions are being stored
   ```bash
   # Check ChromaDB collection count
   curl -s -X GET http://localhost:8000/api/v1/collections | jq '.[0].name, .[0].id'
   ```

2. **Embedding generation failures**: Check Ollama embedding model
   ```bash
   # Test embedding generation
   curl -X POST http://localhost:11434/api/embeddings -d '{"model": "all-minilm", "prompt": "test"}'
   ```

## Advanced Features

### Learning and Adaptation

RAG CLI implements a sophisticated learning system that improves over time:

#### What Gets Learned
- **Platform-specific command syntax** (macOS vs Linux differences)
- **Successful command patterns** for common tasks
- **Error recovery strategies** that worked in the past
- **Multi-step workflow patterns** for complex operations

#### How Learning Works
1. **Execution Storage**: Every command execution session is stored in ChromaDB
2. **Semantic Retrieval**: Similar queries retrieve relevant past executions
3. **Context Integration**: Historical context improves AI decision-making
4. **Pattern Recognition**: AI identifies successful patterns and avoids failed ones

#### Observing Learning
You can observe the AI learning by:
- Running similar commands multiple times
- Watching error correction improve over time
- Noting how initial command suggestions get better
- Seeing platform-specific syntax used correctly

### Command Execution Modes

#### Interactive Mode (Default)
```bash
./rag-cli chat --allow-commands
```
- User approval required for each command
- Shows all commands before execution
- Safest option for untrusted or complex operations

#### Auto-Approve Mode
```bash
./rag-cli chat --allow-commands --auto-approve
```
- Commands execute automatically without user confirmation
- Faster workflow for trusted operations
- **Use with caution** - commands execute immediately

#### Single Prompt Mode
```bash
./rag-cli chat --allow-commands --prompt "your task here"
```
- Execute one task and exit
- Perfect for scripting and automation
- Can be combined with `--auto-approve`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
