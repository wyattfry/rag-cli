package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"rag-cli/internal/embeddings"
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

	// Check if we're in non-interactive mode
	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt != "" {
		allowCommands, _ := cmd.Flags().GetBool("allow-commands")
		autoApprove, _ := cmd.Flags().GetBool("auto-approve")
		return handleSinglePrompt(prompt, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove)
	}

	fmt.Println("RAG CLI Chat - Type 'exit' to quit")
	fmt.Println("=====================================")

	// Get command execution flags
	allowCommands, _ := cmd.Flags().GetBool("allow-commands")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	if allowCommands {
		fmt.Println("[Command execution enabled]")
		if autoApprove {
			fmt.Println("[Auto-approve enabled]")
		}
	}

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

		// Generate embedding for the query
		queryEmbedding, err := embeddingsClient.GenerateEmbedding(input)
		if err != nil {
			fmt.Printf("Warning: Failed to generate embedding: %v\n", err)
			queryEmbedding = nil
		}

		// Retrieve relevant context from vector store
		var context []string
		if queryEmbedding != nil {
			context, err = vectorStore.SearchWithEmbedding(queryEmbedding, 5)
			if err != nil {
				fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
				context = []string{}
			}
		}

		// Generate response using LLM
		response, err := llmClient.GenerateResponse(input, context)
		if err != nil {
			fmt.Printf("Error generating response: %v\n", err)
			continue
		}

		// Process response for commands and execute if needed
		enhancedResponse, err := processResponseWithCommands(response, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove)
		if err != nil {
			fmt.Printf("Error processing commands: %v\n", err)
			continue
		}

		fmt.Printf("AI: %s\n\n", enhancedResponse)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

func handleSinglePrompt(prompt string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, allowCommands bool, autoApprove bool) error {
	// Generate embedding for the query
	queryEmbedding, err := embeddingsClient.GenerateEmbedding(prompt)
	if err != nil {
		fmt.Printf("Warning: Failed to generate embedding: %v\n", err)
		queryEmbedding = nil
	}

	// Retrieve relevant context from vector store
	var context []string
	if queryEmbedding != nil {
		context, err = vectorStore.SearchWithEmbedding(queryEmbedding, 5)
		if err != nil {
			fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
			context = []string{}
		}
	}

	// Generate response using LLM
	response, err := llmClient.GenerateResponse(prompt, context)
	if err != nil {
		return fmt.Errorf("error generating response: %w", err)
	}

	// Process response for commands and execute if needed
	enhancedResponse, err := processResponseWithCommands(response, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove)
	if err != nil {
		return fmt.Errorf("error processing commands: %w", err)
	}

	fmt.Println(enhancedResponse)
	return nil
}

// executeCommand runs a shell command and returns its output
func executeCommand(cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// processResponseWithCommands checks for commands in AI response and executes them
func processResponseWithCommands(response string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, allowCommands bool, autoApprove bool) (string, error) {
	// The response should now be raw shell commands, one per line
	response = strings.TrimSpace(response)
	if response == "" {
		return response, nil
	}

	// Split into individual commands
	commands := strings.Split(response, "\n")
	var validCommands []string
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			validCommands = append(validCommands, cmd)
		}
	}

	if len(validCommands) == 0 {
		return response, nil
	}

	// Check if command execution is allowed
	if !allowCommands {
		return response + "\n\n[Command execution is disabled. Use --allow-commands flag to enable.]", nil
	}

	// Ask user for permission to execute commands (unless auto-approved)
	var permission string
	if autoApprove {
		permission = "y"
		fmt.Printf("\nAuto-approving execution of %d command(s)...\n", len(validCommands))
	} else {
		fmt.Printf("\nThe AI wants to execute the following command(s):\n%s\nDo you want to allow this? (y/n): ", strings.Join(validCommands, "\n"))
		reader := bufio.NewReader(os.Stdin)
		permission, _ = reader.ReadString('\n')
		permission = strings.TrimSpace(strings.ToLower(permission))
	}

	if permission != "y" && permission != "yes" {
		return response, nil
	}

	enhancedResponse := "Commands executed:\n"

	// Execute each command and append results
	for _, cmdStr := range validCommands {
		fmt.Printf("\nExecuting: %s\n", cmdStr)

		output, err := executeCommand(cmdStr)
		if err != nil {
			enhancedResponse += fmt.Sprintf("\n$ %s\nError: %v\n", cmdStr, err)
		} else {
			enhancedResponse += fmt.Sprintf("\n$ %s\n%s", cmdStr, output)
		}
	}

	return enhancedResponse, nil
}
