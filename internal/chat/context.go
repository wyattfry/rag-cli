package chat

import (
	"rag-cli/internal/embeddings"
	"rag-cli/internal/vector"
)

// ContextManager handles retrieval of contextual information for chat sessions
type ContextManager struct {
	embeddingsClient *embeddings.Client
	vectorStore      *vector.ChromaClient
}

// NewContextManager creates a new context manager
func NewContextManager(embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient) *ContextManager {
	return &ContextManager{
		embeddingsClient: embeddingsClient,
		vectorStore:      vectorStore,
	}
}

// GetDocumentContext retrieves relevant context from the document store
func (c *ContextManager) GetDocumentContext(prompt string, maxResults int) ([]string, error) {
	// Generate embedding for the query
	queryEmbedding, err := c.embeddingsClient.GenerateEmbedding(prompt)
	if err != nil {
		return nil, err
	}

	// Retrieve relevant context from vector store
	context, err := c.vectorStore.SearchWithEmbedding(c.vectorStore.DocumentsCollection(), queryEmbedding, maxResults)
	if err != nil {
		return nil, err
	}

	return context, nil
}

// GetHistoricalContext retrieves similar command execution sessions from ChromaDB
func (c *ContextManager) GetHistoricalContext(query string, maxResults int) ([]string, error) {
	// Generate embedding for the query
	queryEmbedding, err := c.embeddingsClient.GenerateEmbedding(query)
	if err != nil {
		return nil, err
	}

	// Search for similar historical command sessions
	historicalContext, err := c.vectorStore.SearchWithEmbedding(c.vectorStore.CommandsCollection(), queryEmbedding, maxResults)
	if err != nil {
		return nil, err
	}

	return historicalContext, nil
}

// GetCombinedContext retrieves both document and historical context
func (c *ContextManager) GetCombinedContext(prompt string, includeHistory bool, maxDocuments, maxHistory int) ([]string, error) {
	// Get document context
	documentContext, err := c.GetDocumentContext(prompt, maxDocuments)
	if err != nil {
		return nil, err
	}

	var allContext []string
	allContext = append(allContext, documentContext...)

	// Get historical context if enabled
	if includeHistory {
		historicalContext, err := c.GetHistoricalContext(prompt, maxHistory)
		if err != nil {
			// Don't fail completely if historical context fails
			return documentContext, nil
		}
		allContext = append(allContext, historicalContext...)
	}

	return allContext, nil
}
