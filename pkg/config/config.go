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
	AutoIndex  AutoIndexConfig  `mapstructure:"auto_index"`
}

type LLMConfig struct {
	Model    string `mapstructure:"model"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
}

type VectorConfig struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	Collection          string `mapstructure:"collection"`           // Main documents collection
	CommandCollection   string `mapstructure:"command_collection"`   // Command execution history
	AutoIndexCollection string `mapstructure:"auto_index_collection"` // Auto-indexed files
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

type AutoIndexConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Extensions      []string `mapstructure:"extensions"`
	MaxFileSize     int64    `mapstructure:"max_file_size"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
	BatchDelay      string   `mapstructure:"batch_delay"`
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
	viper.SetDefault("vector.command_collection", "command_history")
	viper.SetDefault("vector.auto_index_collection", "auto_indexed")
	
	viper.SetDefault("embeddings.model", "all-minilm")
	viper.SetDefault("embeddings.host", "localhost")
	viper.SetDefault("embeddings.port", 11434)
	viper.SetDefault("embeddings.base_url", "http://localhost:11434")
	
	viper.SetDefault("chunker.chunk_size", 1000)
	viper.SetDefault("chunker.chunk_overlap", 200)
	
	// Auto-index defaults
	viper.SetDefault("auto_index.enabled", false)
	viper.SetDefault("auto_index.extensions", []string{".txt", ".md", ".py", ".js", ".go", ".json", ".yaml", ".yml"})
	viper.SetDefault("auto_index.max_file_size", 1048576) // 1MB in bytes
	viper.SetDefault("auto_index.exclude_patterns", []string{".git/*", "node_modules/*", "*.log", "tmp/*", "temp/*", "*.tmp"})
	viper.SetDefault("auto_index.batch_delay", "2s")

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
