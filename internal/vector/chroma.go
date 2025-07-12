package vector

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"rag-cli/pkg/config"
)

type ChromaClient struct {
	baseURL     string
	client      *http.Client
	collections map[string]string // collection name -> collection ID mapping
	config      config.VectorConfig // store config for collection names
}

type Collection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CollectionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Document struct {
	IDs       []string    `json:"ids"`
	Documents []string    `json:"documents"`
	Embeddings [][]float32 `json:"embeddings"`
}

type QueryRequest struct {
	QueryEmbeddings [][]float32 `json:"query_embeddings"`
	NResults        int         `json:"n_results"`
}

type QueryResponse struct {
	IDs       [][]string    `json:"ids"`
	Documents [][]string    `json:"documents"`
	Distances [][]float32   `json:"distances"`
}

// generateUUID generates a simple UUID for ChromaDB
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a simple time-based ID if random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// Set version (4) and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func NewChromaClient(cfg config.VectorConfig) (*ChromaClient, error) {
	client := &ChromaClient{
		baseURL:     fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		collections: make(map[string]string),
		config:      cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize all collections
	collectionNames := []string{
		cfg.Collection,
		cfg.CommandCollection,
		cfg.AutoIndexCollection,
	}

	for _, name := range collectionNames {
		if err := client.createCollection(name); err != nil {
			return nil, fmt.Errorf("failed to create collection %s: %w", name, err)
		}
	}

	return client, nil
}

func (c *ChromaClient) createCollection(name string) error {
	// First try to find existing collection
	if id, err := c.findCollection(name); err == nil {
		c.collections[name] = id
		return nil // Collection found
	}

	// Create new collection
	collection := Collection{Name: name}
	reqBody, err := json.Marshal(collection)
	if err != nil {
		return fmt.Errorf("failed to marshal collection: %w", err)
	}

	resp, err := http.Post(c.baseURL+"/api/v1/collections", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Get the collection ID from response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var collectionResp CollectionResponse
	if err := json.Unmarshal(body, &collectionResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.collections[name] = collectionResp.ID
	return nil
}

func (c *ChromaClient) findCollection(name string) (string, error) {
	resp, err := http.Get(c.baseURL + "/api/v1/collections")
	if err != nil {
		return "", fmt.Errorf("failed to get collections: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var collections []CollectionResponse
	if err := json.Unmarshal(body, &collections); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Find collection by name
	for _, col := range collections {
		if col.Name == name {
			return col.ID, nil
		}
	}

	return "", fmt.Errorf("collection %s not found", name)
}

func (c *ChromaClient) AddDocument(collectionName, id, content string, embedding []float32) error {
	if id == "" {
		id = generateUUID()
	}
	
	collectionID, exists := c.collections[collectionName]
	if !exists {
		return fmt.Errorf("collection %s not found", collectionName)
	}
	
	doc := Document{
		IDs:        []string{id},
		Documents:  []string{content},
		Embeddings: [][]float32{embedding},
	}

	reqBody, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/add", c.baseURL, collectionID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to add document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *ChromaClient) Search(query string, numResults int) ([]string, error) {
	// For now, return empty results since we need embeddings for the query
	// This would need to be implemented with actual query embeddings
	return []string{}, nil
}

func (c *ChromaClient) SearchWithEmbedding(collectionName string, queryEmbedding []float32, numResults int) ([]string, error) {
	collectionID, exists := c.collections[collectionName]
	if !exists {
		return nil, fmt.Errorf("collection %s not found", collectionName)
	}
	
	queryReq := QueryRequest{
		QueryEmbeddings: [][]float32{queryEmbedding},
		NResults:        numResults,
	}

	reqBody, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/query", c.baseURL, collectionID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []string
	if len(queryResp.Documents) > 0 {
		results = queryResp.Documents[0]
	}

	return results, nil
}

// Helper methods to get collection names
func (c *ChromaClient) DocumentsCollection() string {
	return c.config.Collection
}

func (c *ChromaClient) CommandsCollection() string {
	return c.config.CommandCollection
}

func (c *ChromaClient) AutoIndexCollection() string {
	return c.config.AutoIndexCollection
}
