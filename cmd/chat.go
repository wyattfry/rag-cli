package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
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
	// Add auto-index flag for automatic file indexing
	chatCmd.Flags().Bool("auto-index", false, "Automatically index file changes after command execution")
	// Add no-history flag to disable historical context lookup
	chatCmd.Flags().Bool("no-history", false, "Disable historical context lookup (useful for testing)")
}

// showHelp displays help information for the interactive chat
func showHelp() {
	separatorColor.Println(lightRule)
	infoColor.Println("RAG CLI Interactive Chat Help")
	separatorColor.Println(lightRule)
	fmt.Println("Available commands:")
	fmt.Println("  help, ?     - Show this help message")
	fmt.Println("  clear       - Clear the screen")
	fmt.Println("  exit, quit  - Exit the chat")
	fmt.Println("")
	fmt.Println("Features:")
	fmt.Println("  • Use ↑/↓ arrows to navigate command history")
	fmt.Println("  • Ctrl+A to jump to beginning of line")
	fmt.Println("  • Ctrl+E to jump to end of line")
	fmt.Println("  • Ctrl+C to cancel current input")
	fmt.Println("  • Ctrl+D to exit chat")
	fmt.Println("")
	fmt.Println("The AI can execute shell commands if enabled with --allow-commands flag.")
	fmt.Println("You'll be prompted to approve each command before execution.")
	separatorColor.Println(lightRule)
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
		autoIndex, _ := cmd.Flags().GetBool("auto-index")
		noHistory, _ := cmd.Flags().GetBool("no-history")
		return handleSinglePrompt(prompt, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove, autoIndex, noHistory)
	}

	infoColor.Println("RAG CLI Chat - Type 'exit' to quit")
	separatorColor.Println(horizontalRule)

	// Get command execution flags
	allowCommands, _ := cmd.Flags().GetBool("allow-commands")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	autoIndex, _ := cmd.Flags().GetBool("auto-index")
	
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
	
	if allowCommands {
		infoColor.Println("[Command execution enabled]")
		if autoApprove {
			infoColor.Println("[Auto-approve enabled]")
		}
		if autoIndex {
			infoColor.Println("[Auto-indexing enabled]")
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
			context, err = vectorStore.SearchWithEmbedding(vectorStore.DocumentsCollection(), queryEmbedding, 5)
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
		enhancedResponse, err := processResponseWithCommands(response, input, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove, autoIndexer)
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

func handleSinglePrompt(prompt string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, allowCommands bool, autoApprove bool, autoIndex bool, noHistory bool) error {
	// Generate embedding for the query
	queryEmbedding, err := embeddingsClient.GenerateEmbedding(prompt)
	if err != nil {
		fmt.Printf("Warning: Failed to generate embedding: %v\n", err)
		queryEmbedding = nil
	}

	// Retrieve relevant context from vector store
	var context []string
	if queryEmbedding != nil {
		context, err = vectorStore.SearchWithEmbedding(vectorStore.DocumentsCollection(), queryEmbedding, 5)
		if err != nil {
			fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
			context = []string{}
		}
	}

// Get historical command execution context
var historicalContext []string
if !noHistory {
	historicalContext, err = getHistoricalContext(prompt, embeddingsClient, vectorStore)
	if err != nil {
		fmt.Printf("Warning: Failed to retrieve historical context: %v\n", err)
		historicalContext = []string{}
	}
}

	// Combine regular context with historical context
	allContext := append(context, historicalContext...)

	// Initialize auto-indexer if enabled
	var autoIndexer *indexing.AutoIndexer
	if autoIndex {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		
		// Load config for auto-index settings
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
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

	// Generate response using LLM
	response, err := llmClient.GenerateResponse(prompt, allContext)
	if err != nil {
		return fmt.Errorf("error generating response: %w", err)
	}

	// Process response for commands and execute if needed
	enhancedResponse, err := processResponseWithCommands(response, prompt, llmClient, embeddingsClient, vectorStore, allowCommands, autoApprove, autoIndexer)
	if err != nil {
		return fmt.Errorf("error processing commands: %w", err)
	}

	fmt.Println(enhancedResponse)
	return nil
}

// executeCommand runs a shell command and returns its output
// If the command contains pipes, it splits and executes each part separately
// to provide better visibility into intermediate outputs
func executeCommand(cmdStr string) (string, error) {
	// Check if command contains pipes
	if strings.Contains(cmdStr, " | ") {
		return executePipedCommand(cmdStr)
	}
	
	// Simple command execution
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// executePipedCommand handles commands with pipes by executing each part separately
func executePipedCommand(cmdStr string) (string, error) {
	// Split command on pipes
	parts := strings.Split(cmdStr, " | ")
	if len(parts) < 2 {
		// Fallback to normal execution if split didn't work as expected
		cmd := exec.Command("sh", "-c", cmdStr)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("command failed: %w", err)
		}
		return string(output), nil
	}
	
	var currentInput []byte
	var executionDetails strings.Builder
	
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Create command
		cmd := exec.Command("sh", "-c", part)
		
		// If this is not the first command, pipe the previous output as input
		if i > 0 && len(currentInput) > 0 {
			cmd.Stdin = bytes.NewReader(currentInput)
		}
		
		// Execute command and capture both stdout and stderr
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		
		output := stdout.Bytes()
		stderrOutput := stderr.String()
		
		if err != nil {
			// Log details of what succeeded before the failure
			if i > 0 {
				executionDetails.WriteString(fmt.Sprintf("Steps 1-%d succeeded. ", i))
				executionDetails.WriteString(fmt.Sprintf("Step %d failed: %s", i+1, part))
				if stderrOutput != "" {
					executionDetails.WriteString(fmt.Sprintf(" (stderr: %s)", stderrOutput))
				}
				// Include the intermediate output that was successful
				if len(currentInput) > 0 {
					executionDetails.WriteString(fmt.Sprintf("\nIntermediate output from previous steps:\n%s", string(currentInput)))
				}
				return executionDetails.String(), fmt.Errorf("pipe step %d failed: %w", i+1, err)
			} else {
				// For first step failures, include stderr in the error output
				errorOutput := string(output)
				if stderrOutput != "" {
					errorOutput += "\nstderr: " + stderrOutput
				}
				return errorOutput, fmt.Errorf("command failed: %w", err)
			}
		}
		
		// For successful commands, combine stdout and stderr (if stderr has content)
		combinedOutput := output
		if stderrOutput != "" {
			// Include stderr output for successful commands as it may contain useful info
			combinedOutput = append(output, []byte("\nstderr: "+stderrOutput)...)
		}
		
		// Store output for next command in the pipe (only stdout goes to next command)
		currentInput = output
		
		// Log successful step (but don't include in final output unless it's the last step)
		if i < len(parts)-1 {
			executionDetails.WriteString(fmt.Sprintf("Step %d (%s): %d bytes of output\n", i+1, part, len(output)))
		} else {
			// For the last step, return combined output including stderr
			return string(combinedOutput), nil
		}
	}
	
	// This shouldn't be reached, but return currentInput as fallback
	return string(currentInput), nil
}

// processResponseWithCommands checks for commands in AI response and executes them iteratively
func processResponseWithCommands(response string, originalRequest string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, allowCommands bool, autoApprove bool, autoIndexer *indexing.AutoIndexer) (string, error) {
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
		if cmd != "" && isValidCommand(cmd) {
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
	return executeCommandsIteratively(validCommands, originalRequest, llmClient, embeddingsClient, vectorStore, autoApprove, autoIndexer)
}

// executeCommandsIteratively executes commands one by one, allowing AI to refine approach based on results
func executeCommandsIteratively(initialCommands []string, originalRequest string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoApprove bool, autoIndexer *indexing.AutoIndexer) (string, error) {
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
				// Include the actual command output (stderr) in the log for AI context
				if output != "" {
					executionLog.WriteString(fmt.Sprintf("$ %s\n%s\nError: %v\n\n", cmdStr, output, err))
				} else {
					executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", cmdStr, err))
				}
				lastErr = err
				break // Exit the current execution loop if there's an error
			} else {
				outputColor.Printf("%s", output)
				executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", cmdStr, output))
				lastErr = nil
				
				// Auto-index file changes after successful command execution
				if autoIndexer != nil {
					go func() {
						if changedFiles, err := autoIndexer.DetectChanges(); err == nil && len(changedFiles) > 0 {
							if err := autoIndexer.IndexChangedFiles(changedFiles); err != nil {
								fmt.Printf("[Auto-index error: %v]\n", err)
							}
						}
					}()
				}
			}
		}

		 // No need to ask AI for the next steps until the whole queue is executed
		// Evaluate results and get new commands if needed
		nextCommands, shouldContinue, evalErr := evaluateAndGetNextCommands(
			executionLog.String(),
			originalRequest,
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

// evaluateAndGetNextCommands asks AI to evaluate command results using structured decision-making
func evaluateAndGetNextCommands(executionLog string, originalRequest string, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, remainingCommands []string, hadError bool) ([]string, bool, error) {
	// Debug logging
	if err := writeDebugLog("evaluation_debug.log", fmt.Sprintf("=== EVALUATION DEBUG ===\nExecution Log:\n%s\n\nHad Error: %v\nRemaining Commands: %v\n\n", executionLog, hadError, remainingCommands)); err != nil {
		fmt.Printf("Warning: Failed to write debug log: %v\n", err)
	}

	// Step 1: Check if the original goal has been achieved
	goalAchieved, err := checkGoalAchievement(executionLog, originalRequest, llmClient)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check goal achievement: %w", err)
	}

	if goalAchieved {
		if err := writeDebugLog("evaluation_debug.log", "=== GOAL ACHIEVED - STOPPING ===\n\n"); err != nil {
			fmt.Printf("Warning: Failed to write debug log: %v\n", err)
		}
		return nil, false, nil
	}

	// Step 2: If goal not achieved, determine next steps based on current state
	if len(remainingCommands) == 0 {
		// Step 3: No commands queued - determine what to do next
		nextCommands, err := determineNextCommands(executionLog, originalRequest, hadError, llmClient)
		if err != nil {
			return nil, false, fmt.Errorf("failed to determine next commands: %w", err)
		}
		return nextCommands, len(nextCommands) > 0, nil
	} else {
		// Step 4: Commands queued - decide whether to proceed or modify
		queueDecision, newCommands, err := evaluateCommandQueue(executionLog, originalRequest, remainingCommands, hadError, llmClient)
		if err != nil {
			return nil, false, fmt.Errorf("failed to evaluate command queue: %w", err)
		}
		
		switch queueDecision {
		case "proceed":
			return remainingCommands, true, nil
		case "modify":
			return newCommands, len(newCommands) > 0, nil
		case "stop":
			return nil, false, nil
		default:
			return nil, false, nil
		}
	}
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
	if err := vectorStore.AddDocument(vectorStore.CommandsCollection(), sessionID, summary, embedding); err != nil {
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
	contexts, err := vectorStore.SearchWithEmbedding(vectorStore.CommandsCollection(), queryEmbedding, 3)
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

// checkGoalAchievement determines if the original user request has been satisfied
func checkGoalAchievement(executionLog string, originalRequest string, llmClient *llm.Client) (bool, error) {
	var prompt strings.Builder
	prompt.WriteString("You are evaluating whether a user's original request has been satisfied based on command execution results.\n\n")
	prompt.WriteString("Original user request: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nCommand execution log:\n")
	prompt.WriteString(executionLog)
	prompt.WriteString("\n\nYour task is to determine if the commands executed above successfully accomplished the user's original goal.\n")
	prompt.WriteString("Focus only on whether the ORIGINAL REQUEST has been satisfied, not whether more commands could be run.\n\n")
	prompt.WriteString("Examples of satisfied requests:\n")
	prompt.WriteString("- If user asked 'how many files' and 'find . -type f | wc -l' returned '139', the request is SATISFIED\n")
	prompt.WriteString("- If user asked 'list files' and 'ls' showed a directory listing, the request is SATISFIED\n")
	prompt.WriteString("- If user asked 'check disk space' and 'df -h' showed disk usage, the request is SATISFIED\n\n")
	prompt.WriteString("IMPORTANT: You must respond with exactly one of these two words:\n")
	prompt.WriteString("- SATISFIED (if the original request has been completely fulfilled)\n")
	prompt.WriteString("- NOT_SATISFIED (if more work is needed)\n")
	prompt.WriteString("\nDo not provide any explanation, commands, or other text - just the single word.\n")

	response, err := llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToUpper(response))
	if err := writeDebugLog("evaluation_debug.log", fmt.Sprintf("=== GOAL CHECK ===\nResponse: %s\n\n", response)); err != nil {
		fmt.Printf("Warning: Failed to write debug log: %v\n", err)
	}

	return response == "SATISFIED", nil
}

// determineNextCommands decides what commands to run when the queue is empty
func determineNextCommands(executionLog string, originalRequest string, hadError bool, llmClient *llm.Client) ([]string, error) {
	var prompt strings.Builder
	prompt.WriteString("The user's original request has not been satisfied yet. Determine what commands to run next.\n\n")
	prompt.WriteString("Original user request: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nCommand execution log:\n")
	prompt.WriteString(executionLog)
	prompt.WriteString("\n\n")

	if hadError {
		prompt.WriteString("The last command failed. Provide alternative commands to achieve the original goal.\n")
		prompt.WriteString("Do not repeat the same failed command. Use different syntax or approach.\n")
	} else {
		prompt.WriteString("The previous commands succeeded but the original goal hasn't been fully achieved yet.\n")
		prompt.WriteString("Provide the next commands needed to complete the original request.\n")
	}

	prompt.WriteString("\nRespond with shell commands only, one per line. If no more commands are needed, respond with 'NONE'.\n")

	response, err := llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return nil, err
	}

	response = strings.TrimSpace(response)
	if err := writeDebugLog("evaluation_debug.log", fmt.Sprintf("=== NEXT COMMANDS ===\nResponse: %s\n\n", response)); err != nil {
		fmt.Printf("Warning: Failed to write debug log: %v\n", err)
	}

	if strings.ToUpper(response) == "NONE" {
		return nil, nil
	}

	commands := strings.Split(response, "\n")
	var validCommands []string
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" && !strings.HasPrefix(cmd, "#") && isValidCommand(cmd) {
			validCommands = append(validCommands, cmd)
		}
	}

	if len(validCommands) > 0 {
		infoColor.Printf("\nAI suggests next command(s): ")
		for i, cmd := range validCommands {
			if i > 0 {
				fmt.Printf(", ")
			}
			commandColor.Printf("%s", cmd)
		}
		fmt.Println()
	}

	return validCommands, nil
}

// evaluateCommandQueue decides whether to proceed with queued commands or modify them
func evaluateCommandQueue(executionLog string, originalRequest string, remainingCommands []string, hadError bool, llmClient *llm.Client) (string, []string, error) {
	var prompt strings.Builder
	prompt.WriteString("You need to decide whether to proceed with the planned commands or modify the plan.\n\n")
	prompt.WriteString("Original user request: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nCommand execution log:\n")
	prompt.WriteString(executionLog)
	prompt.WriteString("\n\nPlanned remaining commands:\n")
	for _, cmd := range remainingCommands {
		prompt.WriteString(cmd + "\n")
	}

	if hadError {
		prompt.WriteString("\nThe last command failed. You should either:\n")
		prompt.WriteString("- MODIFY: Replace the planned commands with different ones\n")
		prompt.WriteString("- STOP: If the failure means the goal cannot be achieved\n")
	} else {
		prompt.WriteString("\nThe last command succeeded. You should either:\n")
		prompt.WriteString("- PROCEED: Continue with the planned commands as-is\n")
		prompt.WriteString("- MODIFY: Change the planned commands based on new information\n")
		prompt.WriteString("- STOP: If the goal has been achieved and no more commands are needed\n")
	}

	prompt.WriteString("\nRespond with:\n")
	prompt.WriteString("- 'PROCEED' to continue with the planned commands\n")
	prompt.WriteString("- 'MODIFY' followed by new commands (one per line) to replace the plan\n")
	prompt.WriteString("- 'STOP' if no more commands are needed\n")

	response, err := llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return "", nil, err
	}

	response = strings.TrimSpace(response)
	if err := writeDebugLog("evaluation_debug.log", fmt.Sprintf("=== QUEUE DECISION ===\nResponse: %s\n\n", response)); err != nil {
		fmt.Printf("Warning: Failed to write debug log: %v\n", err)
	}

	lines := strings.Split(response, "\n")
	firstLine := strings.TrimSpace(strings.ToUpper(lines[0]))

	switch firstLine {
	case "PROCEED":
		return "proceed", nil, nil
	case "STOP":
		return "stop", nil, nil
	case "MODIFY":
		var newCommands []string
		for i := 1; i < len(lines); i++ {
			cmd := strings.TrimSpace(lines[i])
			if cmd != "" && !strings.HasPrefix(cmd, "#") && isValidCommand(cmd) {
				newCommands = append(newCommands, cmd)
			}
		}
		if len(newCommands) > 0 {
			infoColor.Printf("\nAI modified command queue: ")
			for i, cmd := range newCommands {
				if i > 0 {
					fmt.Printf(", ")
				}
				commandColor.Printf("%s", cmd)
			}
			fmt.Println()
		}
		return "modify", newCommands, nil
	default:
		return "stop", nil, nil
	}
}

// writeDebugLog writes debug information to a log file
func writeDebugLog(filename, content string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = file.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, content))
	return err
}

// isValidCommand checks if a command string is valid for execution
func isValidCommand(cmd string) bool {
	// Remove common invalid patterns
	if cmd == "" {
		return false
	}
	
	// Skip commands that start with shell prompts
	if strings.HasPrefix(cmd, "$") || strings.HasPrefix(cmd, "#") || strings.HasPrefix(cmd, ">") {
		return false
	}
	
	// Skip commands that look like output (all numbers, or common output patterns)
	if strings.TrimSpace(cmd) == "" {
		return false
	}
	
	// Skip pure numeric responses (likely command output)
	if strings.Contains(cmd, "\n") {
		return false
	}
	
	// Skip lines that look like directory listings
	if strings.Contains(cmd, "drwxr-xr-x") || strings.Contains(cmd, "total ") {
		return false
	}
	
	// Skip commands that are just numbers
	if strings.TrimSpace(cmd) != "" {
		if _, err := strconv.Atoi(strings.TrimSpace(cmd)); err == nil {
			return false
		}
	}
	
	// Skip error messages
	if strings.Contains(cmd, "Error:") || strings.Contains(cmd, "command not found") {
		return false
	}
	
	return true
}
