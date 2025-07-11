package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"rag-cli/internal/llm"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"

	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with the AI",
	Long: `Start an interactive chat session with the local AI model.
The AI will use RAG to provide contextual responses based on your indexed documents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChat()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat() error {
	fmt.Println("HI")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Initialize vector store
	vectorStore, err := vector.NewChromaClient(cfg.Vector)
	if err != nil {
		return fmt.Errorf("failed to initialize vector store: %w", err)
	}

	fmt.Println("RAG CLI Chat - Type 'exit' to quit")
	fmt.Println("=====================================")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "exit" || input == "quit" {
			break
		}

		if input == "" {
			continue
		}

		// Retrieve relevant context from vector store
		context, err := vectorStore.Search(input, 5)
		if err != nil {
			fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
			context = []string{}
		}

		// Generate response using LLM
		response, err := llmClient.GenerateResponse(input, context)
		if err != nil {
			fmt.Printf("Error generating response: %v\n", err)
			continue
		}

		fmt.Printf("AI: %s\n\n", response)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}
