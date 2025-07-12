package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	"rag-cli/internal/chat"
	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"
	"rag-cli/pkg/version"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rag-cli",
	Short: "A CLI tool with RAG capabilities using local models",
	Long: `RAG CLI is a command-line tool that provides AI-powered assistance
using Retrieval-Augmented Generation (RAG) with local language models.
It can process documents, generate embeddings, and interact with vector databases.

The AI assistant can execute shell commands, learn from past interactions, and
provide contextual responses based on your indexed documents. It features
intelligent command execution with iterative feedback and error correction.

By default, starts an interactive chat session. Use subcommands for other operations.

EXAMPLES:
  # Start interactive chat (default behavior)
  rag-cli

  # Single prompt with command execution
  rag-cli --prompt "create a backup of my config files"

  # Auto-approve commands (use with caution)
  rag-cli --auto-approve --prompt "show me the largest files"

  # Non-interactive mode without command execution
  rag-cli --prompt "explain how to set up a Go project" --no-history

PREREQUISITES:
  - Ollama running locally (brew install ollama)
  - ChromaDB running in Docker or locally
  - Models: llama3.1:8b (8B+ recommended), all-minilm

CONFIGURATION:
  Create ~/.rag-cli.yaml to customize LLM models, hosts, and other settings.
  See config-example.yaml for reference.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			fmt.Println(version.GetBuildInfo().String())
			return nil
		}
		
		// If no subcommand provided, run chat mode
		return runChat(cmd)
	},
}

// docsCmd generates documentation for all commands
var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation for all commands",
	Long:   "Generate Markdown documentation for all commands and subcommands.",
	Hidden: true, // Hidden from help output
	RunE: func(cmd *cobra.Command, args []string) error {
		docsDir := "./docs"
		
		// Create docs directory if it doesn't exist
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			return fmt.Errorf("failed to create docs directory: %w", err)
		}
		
		// Generate markdown documentation
		if err := doc.GenMarkdownTree(rootCmd, docsDir); err != nil {
			return fmt.Errorf("failed to generate documentation: %w", err)
		}
		
		fmt.Printf("Documentation generated in %s/\n", docsDir)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Add subcommands
	rootCmd.AddCommand(docsCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.rag-cli.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode with detailed logging")
	rootCmd.Flags().BoolP("version", "v", false, "Print version information and build details")
	
	// Chat flags (now at root level)
	rootCmd.Flags().StringP("prompt", "p", "", "Single prompt for non-interactive mode. Execute one task and exit.")
	rootCmd.Flags().Bool("auto-approve", false, "Automatically approve command execution without user confirmation. USE WITH CAUTION - commands execute immediately.")
	rootCmd.Flags().Bool("auto-index", false, "Automatically index file changes after command execution for learning")
	rootCmd.Flags().Bool("no-history", false, "Disable historical context lookup. Useful for testing or when you want fresh responses without past context.")
	
	// Bind flags to viper
	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding debug flag: %v\n", err)
	}
}

func runChat(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Initialize embeddings client
	embeddingsClient, err := embeddings.NewClient(cfg.Embeddings)
	if err != nil {
		return fmt.Errorf("failed to initialize embeddings client: %w", err)
	}

	// Initialize vector store
	vectorStore, err := vector.NewChromaClient(cfg.Vector)
	if err != nil {
		return fmt.Errorf("failed to initialize vector store: %w", err)
	}

	// Get flags
	prompt, _ := cmd.Flags().GetString("prompt")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	autoIndex, _ := cmd.Flags().GetBool("auto-index")
	noHistory, _ := cmd.Flags().GetBool("no-history")

	// Create session config
	sessionConfig := &chat.SessionConfig{
		AutoApprove:     autoApprove,
		AutoIndex:       autoIndex,
		NoHistory:       noHistory,
		MaxAttempts:     cfg.Chat.MaxAttempts,
		MaxOutputLines:  cfg.Chat.MaxOutputLines,
		TruncateOutput:  cfg.Chat.TruncateOutput,
	}

	// Initialize auto-indexer if enabled
	var autoIndexer *indexing.AutoIndexer
	if autoIndex {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		
		// Create auto-index config (override enabled flag from CLI)
		autoIndexConfig := cfg.AutoIndex
		autoIndexConfig.Enabled = true
		
		autoIndexer = indexing.NewAutoIndexer(&autoIndexConfig, embeddingsClient, vectorStore, cwd)
		// Take initial snapshot
		if err := autoIndexer.TakeSnapshot(); err != nil {
			fmt.Printf("Warning: Failed to take initial file snapshot: %v\n", err)
		}
	}

	// Check if we're in non-interactive mode
	if prompt != "" {
		session := chat.NewSession(sessionConfig, llmClient, embeddingsClient, vectorStore, autoIndexer)
		return session.HandlePrompt(prompt)
	}

	// Run interactive session with simple implementation
	simpleSession := chat.NewSimpleSession(sessionConfig, llmClient, embeddingsClient, vectorStore, autoIndexer)
	return simpleSession.Run()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".rag-cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".rag-cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
