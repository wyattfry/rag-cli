package chunker

import "rag-cli/pkg/config"

type Client struct {
	chunkSize    int
	chunkOverlap int
}

func New(cfg config.ChunkerConfig) *Client {
	return &Client{
		chunkSize:    cfg.ChunkSize,
		chunkOverlap: cfg.ChunkOverlap,
	}
}

func (c *Client) ChunkText(text string) ([]string, error) {
	// Dummy implementation: split text into chunks of "chunkSize" bytes
	var chunks []string
	textRunes := []rune(text)
	for i := 0; i < len(textRunes); i += c.chunkSize - c.chunkOverlap {
		end := i + c.chunkSize
		if end > len(textRunes) {
			end = len(textRunes)
		}
		chunks = append(chunks, string(textRunes[i:end]))
		if end == len(textRunes) {
			break
		}
	}
	return chunks, nil
}
