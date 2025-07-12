# Documentation

This directory contains auto-generated documentation for the `rag-cli` command-line tool.

## 📚 Files

- `rag-cli.md` - Main command documentation
- `rag-cli_*.md` - Subcommand documentation
- `rag-cli_completion*.md` - Shell completion documentation

## 🔄 Auto-Generation

Documentation is automatically generated from the CLI command structure using Cobra's built-in documentation generator.

### Local Generation

To regenerate documentation locally:

```bash
./rag-cli docs
```

### Automatic Updates

Documentation is automatically updated via GitHub Actions when:
- Code changes are pushed to the `main` branch
- Files in `cmd/`, `internal/`, `pkg/`, or module files are modified

The workflow:
1. Builds the CLI tool
2. Runs the hidden `docs` command
3. Commits any changes back to the repository

## 📖 Usage

The documentation files are in Markdown format and can be:
- Read directly on GitHub
- Converted to other formats (HTML, PDF, etc.)
- Integrated into documentation sites
- Used as reference for the CLI commands and options

## ⚠️ Note

**Do not edit these files manually** - they are auto-generated and will be overwritten.
Any changes to command documentation should be made in the source code comments and command definitions.
