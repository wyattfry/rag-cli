package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"rag-cli/pkg/config"
)

type Client struct {
	baseURL string
	client  *http.Client
	model   string
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func NewClient(cfg config.EmbeddingsConfig) (*Client, error) {
	return &Client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) GenerateEmbedding(text string) ([]float32, error) {
	req := EmbeddingRequest{
		Model: c.model,
		Input: text,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(c.baseURL+"/api/embed", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert first embedding from float64 to float32
	if len(embResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	embedding := make([]float32, len(embResp.Embeddings[0]))
	for i, v := range embResp.Embeddings[0] {
		embedding[i] = float32(v)
	}

	return embedding, nil
}
