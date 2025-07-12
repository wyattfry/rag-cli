package cmd

import (
	"fmt"
	"os"

	"rag-cli/internal/chat"
	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"

	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with the AI",
	Long: `Start an interactive chat session with the local AI model.
The AI will use RAG to provide contextual responses based on your indexed documents.

For non-interactive use, provide a prompt with the --prompt flag.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChat(cmd)
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add non-interactive prompt flag
	chatCmd.Flags().StringP("prompt", "p", "", "Single prompt for non-interactive mode")
	// Add command execution flag
	chatCmd.Flags().BoolP("allow-commands", "c", false, "Allow AI to execute shell commands")
	// Add auto-approve flag for non-interactive execution
	chatCmd.Flags().Bool("auto-approve", false, "Automatically approve command execution (use with caution)")
	// Add auto-index flag for automatic file indexing
	chatCmd.Flags().Bool("auto-index", false, "Automatically index file changes after command execution")
	// Add no-history flag to disable historical context lookup
	chatCmd.Flags().Bool("no-history", false, "Disable historical context lookup (useful for testing)")
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
	allowCommands, _ := cmd.Flags().GetBool("allow-commands")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	autoIndex, _ := cmd.Flags().GetBool("auto-index")
	noHistory, _ := cmd.Flags().GetBool("no-history")

	// Create session config
	sessionConfig := &chat.SessionConfig{
		AllowCommands: allowCommands,
		AutoApprove:   autoApprove,
		AutoIndex:     autoIndex,
		NoHistory:     noHistory,
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

	// Run interactive session
	interactiveSession, err := chat.NewInteractiveSession(sessionConfig, llmClient, embeddingsClient, vectorStore, autoIndexer)
	if err != nil {
		return fmt.Errorf("failed to initialize interactive session: %w", err)
	}
	defer interactiveSession.Close()

	return interactiveSession.Run()
}
