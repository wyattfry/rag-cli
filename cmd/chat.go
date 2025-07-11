package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Color and styling setup
var (
	// User interface colors
	userPromptColor = color.New(color.FgCyan, color.Bold)
	aiResponseColor = color.New(color.FgGreen)
	commandColor    = color.New(color.FgYellow, color.Bold)
	outputColor     = color.New(color.FgWhite)
	errorColor      = color.New(color.FgRed, color.Bold)
	infoColor       = color.New(color.FgBlue)
	separatorColor  = color.New(color.FgMagenta)

	// Styling characters
	horizontalRule = strings.Repeat("─", 60)
	lightRule      = strings.Repeat("·", 40)
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

	infoColor.Println("RAG CLI Chat - Type 'exit' to quit")
	separatorColor.Println(horizontalRule)

	// Get command execution flags
	allowCommands, _ := cmd.Flags().GetBool("allow-commands")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	if allowCommands {
		infoColor.Println("[Command execution enabled]")
		if autoApprove {
			infoColor.Println("[Auto-approve enabled]")
		}
	}

	// Set up readline for interactive input
	rl, err := readline.NewEx(&readline.Config{
		Prompt:              userPromptColor.Sprintf("\u003e "),
		HistoryFile:         filepath.Join(os.TempDir(), "ragcli_history.tmp"),
		InterruptPrompt:     "",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: func(r rune) (rune, bool) { return r, true },
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	// Main interactive loop
	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "exit" || input == "quit" {
			infoColor.Println("Goodbye!")
			break
		}

		if input == "" {
			continue
		}

		// Handle special commands
		if input == "help" || input == "?" {
			showHelp()
			continue
		}

		if input == "clear" {
			// Clear screen
			fmt.Print("\033[2J\033[H")
			infoColor.Println("RAG CLI Chat - Type 'exit' to quit")
			separatorColor.Println(horizontalRule)
			continue
		}

		// Generate embedding for the query
		queryEmbedding, err := embeddingsClient.GenerateEmbedding(input)
		if err != nil {
			errorColor.Printf("Warning: Failed to generate embedding: %v\n", err)
			queryEmbedding = nil
		}

		// Retrieve relevant context from vector store
		var context []string
		if queryEmbedding != nil {
			context, err = vectorStore.SearchWithEmbedding(queryEmbedding, 5)
			if err != nil {
				errorColor.Printf("Warning: Failed to retrieve context: %v\n", err)
				context = []string{}
			}
		}

		// Generate response using LLM
		response, err := llmClient.GenerateResponse(input, context)
		if err != nil {
			errorColor.Printf("Error generating response: %v\n", err)
			continue
		}

		// Process response for commands and execute if needed
		enhancedResponse, err := processResponseWithCommands(response, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove)
		if err != nil {
			errorColor.Printf("Error processing commands: %v\n", err)
			continue
		}

		separatorColor.Println(horizontalRule)
		aicmd := fmt.Sprintf("AI: %s", enhancedResponse)
		aiResponseColor.Println(aicmd)
		separatorColor.Println(horizontalRule)
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

	// Get historical command execution context
	historicalContext, err := getHistoricalContext(prompt, embeddingsClient, vectorStore)
	if err != nil {
		fmt.Printf("Warning: Failed to retrieve historical context: %v\n", err)
		historicalContext = []string{}
	}

	// Combine regular context with historical context
	allContext := append(context, historicalContext...)

	// Generate response using LLM
	response, err := llmClient.GenerateResponse(prompt, allContext)
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

// processResponseWithCommands checks for commands in AI response and executes them iteratively
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
		infoColor.Printf("\nAuto-approving execution of %d command(s)...\n", len(validCommands))
	} else {
		infoColor.Printf("\nThe AI wants to execute the following command(s):\n")
		separatorColor.Println(lightRule)
		for _, cmd := range validCommands {
			commandColor.Printf("$ %s\n", cmd)
		}
		separatorColor.Println(lightRule)
		userPromptColor.Printf("Do you want to allow this? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		permission, _ = reader.ReadString('\n')
		permission = strings.TrimSpace(strings.ToLower(permission))
	}

	if permission != "y" && permission != "yes" {
		return response, nil
	}

	// Execute commands iteratively with feedback
	return executeCommandsIteratively(validCommands, llmClient, embeddingsClient, vectorStore, autoApprove)
}

// executeCommandsIteratively executes commands one by one, allowing AI to refine approach based on results
func executeCommandsIteratively(initialCommands []string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoApprove bool) (string, error) {
	const maxAttempts = 3
	var executionLog strings.Builder
	var commandQueue []string

	// Start with initial commands
	commandQueue = append(commandQueue, initialCommands...)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts && len(commandQueue) > 0; attempt++ {
		if attempt > 1 {
			infoColor.Printf("\nAttempt %d/%d\n", attempt, maxAttempts)
		}

		// Execute all commands in the queue
		for len(commandQueue) > 0 {
			cmdStr := commandQueue[0]
			commandQueue = commandQueue[1:] // Remove executed command
			
			commandColor.Printf("\nExecuting: %s\n", cmdStr)
			
			output, err := executeCommand(cmdStr)
			if err != nil {
				errorColor.Printf("Error: %v\n", err)
				executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", cmdStr, err))
				lastErr = err
				break // Exit the current execution loop if there's an error
			} else {
				outputColor.Printf("%s", output)
				executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", cmdStr, output))
				lastErr = nil
			}
		}

		 // No need to ask AI for the next steps until the whole queue is executed
		// Evaluate results and get new commands if needed
		nextCommands, shouldContinue, evalErr := evaluateAndGetNextCommands(
			executionLog.String(),
			llmClient,
			embeddingsClient,
			vectorStore,
			commandQueue,
			lastErr != nil,
		)

		if evalErr != nil {
			fmt.Printf("Error evaluating results: %v\n", evalErr)
			break
		}

		if !shouldContinue {
			break
		}

		// Replace command queue with new commands
		commandQueue = nextCommands
	}

	if len(commandQueue) > 0 {
		executionLog.WriteString(fmt.Sprintf("\nMax attempts (%d) reached. Remaining commands not executed.\n", maxAttempts))
	}

	// Store the execution session in ChromaDB for future learning
	if err := storeExecutionSession(executionLog.String(), llmClient, embeddingsClient, vectorStore); err != nil {
		fmt.Printf("Warning: Failed to store execution session: %v\n", err)
	}

	return executionLog.String(), nil
}

// evaluateAndGetNextCommands asks AI to evaluate command results and determine next steps
func evaluateAndGetNextCommands(executionLog string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, remainingCommands []string, hadError bool) ([]string, bool, error) {
	// Build evaluation prompt
	var evalPrompt strings.Builder
	evalPrompt.WriteString("Based on the command execution results below, determine if the current step succeeded or if a different approach is needed.\n\n")
	evalPrompt.WriteString("Execution log:\n")
	evalPrompt.WriteString(executionLog)
	
	if len(remainingCommands) > 0 {
		evalPrompt.WriteString("\nPreviously planned remaining commands (may need to be reconsidered):\n")
		for _, cmd := range remainingCommands {
			evalPrompt.WriteString(cmd + "\n")
		}
	}
	
	if hadError {
		evalPrompt.WriteString("\nThe last command failed. You should try a different approach to achieve the same goal. ")
		evalPrompt.WriteString("Use the appropriate command syntax for the current system environment and adapt your strategy based on what you learned from the failure.\n")
	}
	
	evalPrompt.WriteString("\nIf the current step succeeded and no further commands are needed, respond with: DONE\n")
	evalPrompt.WriteString("If you need to execute different/additional commands, respond with only the shell command(s), one per line.\n")
	evalPrompt.WriteString("If the previous command failed, provide an alternative command(s) that will achieve the same goal using the correct syntax for this system.\n")
	evalPrompt.WriteString("Completely reevaluate your approach - don't just retry the same failed command.\n")

	// Get AI's evaluation
	response, err := llmClient.GenerateResponse(evalPrompt.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get AI evaluation: %w", err)
	}

	response = strings.TrimSpace(response)

	// Check if AI thinks we're done
	if strings.ToUpper(response) == "DONE" {
		return nil, false, nil
	}

	// Parse new commands
	commands := strings.Split(response, "\n")
	var validCommands []string
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" && !strings.HasPrefix(cmd, "#") { // Skip comments
			validCommands = append(validCommands, cmd)
		}
	}

	if len(validCommands) == 0 {
		return nil, false, nil
	}

	infoColor.Printf("\nAI suggests next command(s): ")
	for i, cmd := range validCommands {
		if i > 0 {
			fmt.Printf(", ")
		}
		commandColor.Printf("%s", cmd)
	}
	fmt.Println()
	return validCommands, true, nil
}

// storeExecutionSession stores the command execution session in ChromaDB for future learning
func storeExecutionSession(executionLog string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient) error {
	// Create a summary of the execution session
	summary := fmt.Sprintf("Command execution session:\n%s", executionLog)

	// Generate embedding for the execution session
	embedding, err := embeddingsClient.GenerateEmbedding(summary)
	if err != nil {
		return fmt.Errorf("failed to generate embedding for execution session: %w", err)
	}

	// Store in ChromaDB with a unique ID
	sessionID := fmt.Sprintf("cmd_session_%d", time.Now().Unix())
	if err := vectorStore.AddDocument(sessionID, summary, embedding); err != nil {
		return fmt.Errorf("failed to store execution session: %w", err)
	}

	return nil
}

// getHistoricalContext retrieves similar command execution sessions from ChromaDB
func getHistoricalContext(query string, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient) ([]string, error) {
	// Generate embedding for the query
	queryEmbedding, err := embeddingsClient.GenerateEmbedding(query)
	if err != nil {
		return nil, err
	}

	// Search for similar execution sessions
	contexts, err := vectorStore.SearchWithEmbedding(queryEmbedding, 3)
	if err != nil {
		return nil, err
	}

	// Filter for command execution sessions
	var commandContexts []string
	for _, ctx := range contexts {
		if strings.Contains(ctx, "Command execution session:") {
			commandContexts = append(commandContexts, ctx)
		}
	}

	return commandContexts, nil
}
