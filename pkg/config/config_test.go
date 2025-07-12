package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that config loads successfully and has expected types
	// Note: actual values may be overridden by user config files
	if cfg.LLM.Model == "" {
		t.Error("Expected LLM model to be set")
	}

	if cfg.Vector.Host != "localhost" {
		t.Errorf("Expected default vector host to be 'localhost', got '%s'", cfg.Vector.Host)
	}

	if cfg.Vector.Port != 8000 {
		t.Errorf("Expected default vector port to be 8000, got %d", cfg.Vector.Port)
	}

	if cfg.Chunker.ChunkSize != 1000 {
		t.Errorf("Expected default chunk size to be 1000, got %d", cfg.Chunker.ChunkSize)
	}

	if cfg.Chunker.ChunkOverlap != 200 {
		t.Errorf("Expected default chunk overlap to be 200, got %d", cfg.Chunker.ChunkOverlap)
	}
}
