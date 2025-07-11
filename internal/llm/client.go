package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"rag-cli/internal/system"
	"rag-cli/pkg/config"
)

type Client struct {
	baseURL    string
	client     *http.Client
	model      string
	systemInfo *system.SystemInfo
	sysOnce    sync.Once
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

// getSystemInfo returns cached system information, detecting it once
func (c *Client) getSystemInfo() *system.SystemInfo {
	c.sysOnce.Do(func() {
		c.systemInfo = system.DetectSystemInfo()
	})
	return c.systemInfo
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
	
	// Get system information
	sysInfo := c.getSystemInfo()
	
	if len(context) > 0 {
		prompt.WriteString("Context information:\n")
		for i, ctx := range context {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
		}
		prompt.WriteString("\n")
	}
	
	// Add system environment information
	prompt.WriteString(sysInfo.GetCommandSyntaxHints())
	prompt.WriteString("\n")
	
	// Main instructions
	prompt.WriteString("You are a command-line assistant. When a user asks you to perform a task, respond with ONLY the shell command(s) needed to complete that task. ")
	prompt.WriteString("Do not include any markdown formatting, explanations, shell prompts ($, #, >), or other text. ")
	prompt.WriteString("Output only the raw shell command(s), one per line if multiple commands are needed.\n\n")
	
	// System-specific guidance
	prompt.WriteString("IMPORTANT GUIDELINES:\n")
	prompt.WriteString("1. Use the command syntax appropriate for the detected system environment above\n")
	prompt.WriteString("2. Before performing system-specific operations, consider detecting system properties if needed\n")
	prompt.WriteString("3. Use only the tools listed as available in the environment\n")
	prompt.WriteString("4. If you need to detect system properties first, use appropriate detection commands\n\n")
	
	// Add system detection commands as reference
	detectionCommands := sysInfo.GetSystemDetectionCommands()
	if len(detectionCommands) > 0 {
		prompt.WriteString("System detection commands you can use if needed:\n")
		for _, cmd := range detectionCommands {
			prompt.WriteString(fmt.Sprintf("- %s\n", cmd))
		}
		prompt.WriteString("\n")
	}
	
	// Examples based on detected system
	prompt.WriteString("Examples for your system (output ONLY the command, no $ or other symbols):\n")
	prompt.WriteString("User: create a file called hello.txt with content 'hello world'\n")
	prompt.WriteString("Assistant: echo 'hello world' > hello.txt\n\n")
	
	prompt.WriteString("User: list all files in current directory\n")
	prompt.WriteString("Assistant: ls -la\n\n")
	
	// Add system-specific example
	if sysInfo.Capabilities["stat"] == "BSD" {
		prompt.WriteString("User: show file size in bytes\n")
		prompt.WriteString("Assistant: stat -f %z filename\n\n")
	} else if sysInfo.Capabilities["stat"] == "GNU" {
		prompt.WriteString("User: show file size in bytes\n")
		prompt.WriteString("Assistant: stat -c %s filename\n\n")
	}
	
	prompt.WriteString("User request: ")
	prompt.WriteString(query)
	
	return prompt.String()
}
