package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	LLM        LLMConfig        `mapstructure:"llm"`
	Vector     VectorConfig     `mapstructure:"vector"`
	Embeddings EmbeddingsConfig `mapstructure:"embeddings"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"`
}

type LLMConfig struct {
	Model    string `mapstructure:"model"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
}

type VectorConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
}

type EmbeddingsConfig struct {
	Model   string `mapstructure:"model"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

type ChunkerConfig struct {
	ChunkSize    int `mapstructure:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap"`
}

func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("llm.model", "granite-code:3b")
	viper.SetDefault("llm.host", "localhost")
	viper.SetDefault("llm.port", 11434)
	viper.SetDefault("llm.base_url", "http://localhost:11434")
	
	viper.SetDefault("vector.host", "localhost")
	viper.SetDefault("vector.port", 8000)
	viper.SetDefault("vector.collection", "documents")
	
	viper.SetDefault("embeddings.model", "all-minilm")
	viper.SetDefault("embeddings.host", "localhost")
	viper.SetDefault("embeddings.port", 11434)
	viper.SetDefault("embeddings.base_url", "http://localhost:11434")
	
	viper.SetDefault("chunker.chunk_size", 1000)
	viper.SetDefault("chunker.chunk_overlap", 200)

	// Try to read config file
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".rag-cli.yaml")
	if _, err := os.Stat(configPath); err == nil {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
