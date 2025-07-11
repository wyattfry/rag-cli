package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"rag-cli/pkg/config"
)

type Client struct {
	baseURL string
	client  *http.Client
	model   string
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewClient(cfg config.LLMConfig) (*Client, error) {
	return &Client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) GenerateResponse(query string, context []string) (string, error) {
	// Build prompt with context
	prompt := c.buildPrompt(query, context)
	
	// Prepare request
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	resp, err := http.Post(c.baseURL+"/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return genResp.Response, nil
}

func (c *Client) buildPrompt(query string, context []string) string {
	var prompt strings.Builder
	
	if len(context) > 0 {
		prompt.WriteString("Context information:\n")
		for i, ctx := range context {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
		}
		prompt.WriteString("\n")
	}
	
	prompt.WriteString("Based on the context above, please answer the following question:\n")
	prompt.WriteString(query)
	
	return prompt.String()
}
