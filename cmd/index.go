package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"rag-cli/internal/chunker"
	"rag-cli/internal/embeddings"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"
)

var (
	indexRecursive bool
	indexFormats   []string
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index documents for RAG",
	Long: `Index documents by chunking them, generating embeddings, and storing them in the vector database.
This enables the AI to use these documents as context for responses and improves the quality
of AI-generated answers by providing relevant background information.

The indexing process:
1. Scans files in the specified directory (or current directory if none specified)
2. Chunks large documents into manageable pieces (default: 1000 chars with 200 char overlap)
3. Generates embeddings for each chunk using the configured embedding model
4. Stores chunks and embeddings in ChromaDB for fast semantic search

Supported file formats: txt, md, go, py, js, ts, json, yaml, yml (configurable)

EXAMPLES:
  # Index current directory (non-recursive)
  rag-cli index

  # Index specific directory recursively
  rag-cli index -r /path/to/docs

  # Index with specific file formats
  rag-cli index -f txt,md,go /path/to/project

  # Index documentation recursively with multiple formats
  rag-cli index -r -f md,txt,rst ~/projects/my-docs`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		return runIndex(path)
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	
	indexCmd.Flags().BoolVarP(&indexRecursive, "recursive", "r", false, "Index directories recursively, including all subdirectories")
	indexCmd.Flags().StringSliceVarP(&indexFormats, "formats", "f", []string{"txt", "md", "go", "py", "js", "ts", "json", "yaml", "yml"}, "Comma-separated list of file extensions to index (without dots)")
}

func runIndex(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize components
	embeddingClient, err := embeddings.NewClient(cfg.Embeddings)
	if err != nil {
		return fmt.Errorf("failed to initialize embedding client: %w", err)
	}

	vectorStore, err := vector.NewChromaClient(cfg.Vector)
	if err != nil {
		return fmt.Errorf("failed to initialize vector store: %w", err)
	}

	chunkerClient := chunker.New(cfg.Chunker)

	// Get files to index
	files, err := getFilesToIndex(path, indexFormats, indexRecursive)
	if err != nil {
		return fmt.Errorf("failed to get files to index: %w", err)
	}

	fmt.Printf("Found %d files to index\n", len(files))

	// Process each file
	for i, file := range files {
		fmt.Printf("Processing file %d/%d: %s\n", i+1, len(files), file)
		
		if err := processFile(file, chunkerClient, embeddingClient, vectorStore); err != nil {
			fmt.Printf("Error processing file %s: %v\n", file, err)
			continue
		}
	}

	fmt.Println("Indexing complete!")
	return nil
}

func getFilesToIndex(path string, formats []string, recursive bool) ([]string, error) {
	var files []string
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if !recursive && filePath != path {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file format is in the allowed list
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
		for _, format := range formats {
			if ext == format {
				files = append(files, filePath)
				break
			}
		}

		return nil
	})

	return files, err
}

func processFile(filePath string, chunkerClient *chunker.Client, embeddingClient *embeddings.Client, vectorStore *vector.ChromaClient) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Chunk the content
	chunks, err := chunkerClient.ChunkText(string(content))
	if err != nil {
		return fmt.Errorf("failed to chunk text: %w", err)
	}

	// Generate embeddings for each chunk
	for i, chunk := range chunks {
		embedding, err := embeddingClient.GenerateEmbedding(chunk)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}

		// Store in vector database with empty ID to auto-generate UUID
		if err := vectorStore.AddDocument(vectorStore.DocumentsCollection(), "", chunk, embedding); err != nil {
			return fmt.Errorf("failed to store document in vector database: %w", err)
		}
	}

	return nil
}
