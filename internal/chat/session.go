package chat

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"

	"github.com/fatih/color"
)

// SessionConfig holds configuration for a chat session
type SessionConfig struct {
	AutoApprove       bool
	AutoIndex         bool
	NoHistory         bool
	MaxAttempts       int
	MaxOutputLines    int
	TruncateOutput    bool
}

// Session represents an interactive or single-prompt chat session
type Session struct {
	config           *SessionConfig
	llmClient        *llm.Client
	embeddingsClient *embeddings.Client
	vectorStore      *vector.ChromaClient
	autoIndexer      *indexing.AutoIndexer
	
	executor        *CommandExecutor
	validator       *CommandValidator
	evaluator       *AIEvaluator
	contextManager  *ContextManager
	
	// UI colors
	commandColor    *color.Color
	outputColor     *color.Color
	errorColor      *color.Color
	infoColor       *color.Color
}

// NewSession creates a new chat session
func NewSession(config *SessionConfig, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoIndexer *indexing.AutoIndexer) *Session {
	return &Session{
		config:           config,
		llmClient:        llmClient,
		embeddingsClient: embeddingsClient,
		vectorStore:      vectorStore,
		autoIndexer:      autoIndexer,
		
		executor:       NewCommandExecutor(),
		validator:      NewCommandValidator(),
		evaluator:      NewAIEvaluator(llmClient, embeddingsClient, vectorStore),
		contextManager: NewContextManager(embeddingsClient, vectorStore),
		
		// Initialize UI colors
		commandColor: color.New(color.FgYellow, color.Bold),
		outputColor:  color.New(color.FgWhite),
		errorColor:   color.New(color.FgRed, color.Bold),
		infoColor:    color.New(color.FgBlue),
	}
}

// HandlePrompt processes a single prompt (for non-interactive mode)
func (s *Session) HandlePrompt(prompt string) error {
	// Get combined context
	context, err := s.contextManager.GetCombinedContext(prompt, !s.config.NoHistory, 5, 3)
	if err != nil {
		fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
		context = []string{}
	}

	// Generate response using LLM
	response, err := s.llmClient.GenerateResponse(prompt, context)
	if err != nil {
		return fmt.Errorf("error generating response: %w", err)
	}

	// Process response for commands and execute if needed
	enhancedResponse, err := s.processResponseWithCommands(response, prompt)
	if err != nil {
		return fmt.Errorf("error processing commands: %w", err)
	}

	fmt.Println(enhancedResponse)
	return nil
}

// processResponseWithCommands checks for commands in AI response and executes them iteratively
func (s *Session) processResponseWithCommands(response string, originalRequest string) (string, error) {
	// Parse commands from response
	validCommands := s.validator.ParseCommands(response)
	if len(validCommands) == 0 {
		return response, nil
	}

	// Commands are always allowed in chat mode

	// Execute commands iteratively with feedback (approval happens per command now)
	return s.executeCommandsIteratively(validCommands, originalRequest)
}

// requestPermission asks the user for permission to execute a single command
func (s *Session) requestPermission(command string) bool {
	// Generate a human-friendly explanation of what this command does
	explanation := s.generateCommandExplanation(command)
	if explanation != "" {
		s.infoColor.Printf("\n%s\n", explanation)
	} else {
		s.infoColor.Printf("\nI need to run the following command:\n")
	}
	
	lightRule := strings.Repeat("·", 40)
	fmt.Println(lightRule)
	s.commandColor.Printf("$ %s\n", command)
	fmt.Println(lightRule)
	fmt.Printf("Do you want to allow this? (Y/n): ")
	
	reader := bufio.NewReader(os.Stdin)
	permission, _ := reader.ReadString('\n')
	permission = strings.TrimSpace(strings.ToLower(permission))
	
	// Default to yes if user just presses Enter (empty string)
	// Only deny if user explicitly types "n" or "no"
	return permission == "" || permission == "y" || permission == "yes"
}

// generateCommandExplanation creates a human-friendly explanation of what a command does
func (s *Session) generateCommandExplanation(command string) string {
	// Simple pattern-based explanations for common commands
	command = strings.TrimSpace(command)
	
	if strings.HasPrefix(command, "uname") {
		if strings.Contains(command, "-a") {
			return "First, I need to check the system information to identify your operating system."
		} else if strings.Contains(command, "-p") {
			return "Now, I need to check the processor architecture."
		}
		return "I need to check system information."
	}
	
	if strings.HasPrefix(command, "sw_vers") {
		return "Next, I need to get the detailed macOS version information."
	}
	
	if strings.Contains(command, "printenv") && strings.Contains(command, "SHELL") {
		return "I need to check your environment variables to find out what shell you're using."
	}
	
	if strings.HasPrefix(command, "date") {
		if strings.Contains(command, "+") {
			return "I need to get the current date and time in a specific format."
		}
		return "I need to check the current date and time."
	}
	
	if strings.HasPrefix(command, "ipconfig") {
		return "I need to check your local IP address."
	}
	
	if strings.HasPrefix(command, "ifconfig") {
		return "I need to check your network interface configuration."
	}
	
	if strings.Contains(command, "curl") && (strings.Contains(command, "ifconfig.me") || strings.Contains(command, "ipinfo.io")) {
		return "I need to check your external/public IP address."
	}
	
	if strings.HasPrefix(command, "ls") {
		return "I need to list the files and directories here."
	}
	
	if strings.HasPrefix(command, "find") {
		return "I need to search for files matching your criteria."
	}
	
	if strings.HasPrefix(command, "grep") {
		return "I need to search through the output for specific information."
	}
	
	// For pipe commands, explain the overall goal
	if strings.Contains(command, "|") {
		return "I need to run a command and filter its output to get the information you requested."
	}
	
	// Default fallback
	return ""
}

// truncateOutputForDisplay truncates long output for better interactive experience
// while preserving the full output for AI processing
func (s *Session) truncateOutputForDisplay(output string) string {
	if !s.config.TruncateOutput {
		return output
	}
	
	lines := strings.Split(output, "\n")
	totalLines := len(lines)
	maxLines := s.config.MaxOutputLines
	
	// If output is short enough, return as-is
	if totalLines <= maxLines {
		return output
	}
	
	// Calculate how many lines to show from beginning and end
	headLines := maxLines / 2
	tailLines := maxLines - headLines
	
	// Build truncated output
	var result strings.Builder
	
	// Add first N lines
	for i := 0; i < headLines && i < totalLines; i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}
	
	// Add truncation indicator
	skippedLines := totalLines - headLines - tailLines
	if skippedLines > 0 {
		result.WriteString(fmt.Sprintf("\n... [%d lines omitted] ...\n\n", skippedLines))
	}
	
	// Add last N lines
	startIdx := totalLines - tailLines
	for i := startIdx; i < totalLines; i++ {
		result.WriteString(lines[i])
		if i < totalLines-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// executeCommandsIteratively executes commands one by one, allowing AI to refine approach based on results
func (s *Session) executeCommandsIteratively(initialCommands []string, originalRequest string) (string, error) {
	maxAttempts := s.config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3 // fallback default if not set or invalid
	}
	var executionLog strings.Builder
	var commandQueue []string

	// Start with initial commands
	commandQueue = append(commandQueue, initialCommands...)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts && len(commandQueue) > 0; attempt++ {
		// Only show attempt number when we're actually retrying due to failures
		if attempt > 1 {
			s.infoColor.Printf("\nRetry attempt %d/%d\n", attempt, maxAttempts)
		}

		// Execute all commands in the queue
		for len(commandQueue) > 0 {
			cmdStr := commandQueue[0]
			commandQueue = commandQueue[1:] // Remove executed command
			
			// Ask for permission for each command (unless auto-approved)
			if !s.config.AutoApprove {
				if !s.requestPermission(cmdStr) {
					s.infoColor.Printf("Command execution cancelled by user\n")
					return "Command execution cancelled by user.", nil // Return early when user denies
				}
			} else {
				s.infoColor.Printf("\nAuto-approving command: %s\n", cmdStr)
			}
			
			s.commandColor.Printf("\nExecuting: %s\n", cmdStr)
			
			output, err := s.executor.Execute(cmdStr)
			if err != nil {
				s.errorColor.Printf("Error: %v\n", err)
				// Show failure feedback immediately
				s.errorColor.Printf("\n❌ Command failed\n")
				// Include the actual command output (stderr) in the log for AI context
				if output != "" {
					executionLog.WriteString(fmt.Sprintf("$ %s\n%s\nError: %v\n\n", cmdStr, output, err))
				} else {
					executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", cmdStr, err))
				}
				lastErr = err
				break // Exit the current execution loop if there's an error
			} else {
				// Truncate output for display but preserve full output for AI
				displayOutput := s.truncateOutputForDisplay(output)
				s.outputColor.Printf("%s", displayOutput)
				
				// Show success feedback immediately after successful command
				successColor := color.New(color.FgGreen, color.Bold)
				successColor.Printf("\n✅ Command completed successfully\n")
				
				// Store full output in execution log for AI processing
				executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", cmdStr, output))
				lastErr = nil
				
				// Auto-index file changes after successful command execution
				if s.autoIndexer != nil {
					go func() {
						if changedFiles, err := s.autoIndexer.DetectChanges(); err == nil && len(changedFiles) > 0 {
							if err := s.autoIndexer.IndexChangedFiles(changedFiles); err != nil {
								fmt.Printf("[Auto-index error: %v]\n", err)
							}
						}
					}()
				}
			}
		}

		// Evaluate results and get new commands if needed
		nextCommands, shouldContinue, evalErr := s.evaluator.EvaluateAndGetNextCommands(
			executionLog.String(),
			originalRequest,
			commandQueue,
			lastErr != nil,
		)

		if evalErr != nil {
			fmt.Printf("Error evaluating results: %v\n", evalErr)
			break
		}

		if !shouldContinue {
			// Check if we have a successful result to present
			if lastErr == nil && len(commandQueue) == 0 {
				// Generate a final human-readable answer
				finalAnswer, err := s.evaluator.GenerateFinalAnswer(executionLog.String(), originalRequest)
				if err == nil && finalAnswer != "" {
					// Return the final answer instead of the raw execution log
					return finalAnswer, nil
				}
				s.infoColor.Printf("\nTask completed successfully!\n")
			}
			break
		}

		// Provide feedback about what happened and what's next
		if len(nextCommands) > 0 && attempt > 1 {
			if lastErr != nil {
				s.errorColor.Printf("\n❌ That didn't seem to work, let me try something else...\n")
			} else {
				// Use green color for success messages
				successColor := color.New(color.FgGreen, color.Bold)
				successColor.Printf("\n✅ That seemed to work, moving on to the next planned command...\n")
			}
		}

		// Replace command queue with new commands
		commandQueue = nextCommands
		
		// Show AI's decision to modify commands
		if len(nextCommands) > 0 && attempt > 1 {
			s.infoColor.Printf("\nAI suggests next command(s): ")
			for i, cmd := range nextCommands {
				if i > 0 {
					fmt.Printf(", ")
				}
				s.commandColor.Printf("%s", cmd)
			}
			fmt.Println()
		}
	}

	if len(commandQueue) > 0 {
		executionLog.WriteString(fmt.Sprintf("\nMax attempts (%d) reached. Remaining commands not executed.\n", maxAttempts))
	}

	// Store the execution session in ChromaDB for future learning
	if err := s.evaluator.StoreExecutionSession(executionLog.String()); err != nil {
		fmt.Printf("Warning: Failed to store execution session: %v\n", err)
	}
	
	// Debug log the evaluation process (always enabled for debugging)
	if err := WriteDebugLog("evaluation_debug.log", fmt.Sprintf("EVALUATION SESSION:\nOriginal Request: %s\nExecution Log:\n%s\n=== END SESSION ===\n", originalRequest, executionLog.String())); err != nil {
		// Don't fail on debug log errors, just continue
	}

	return executionLog.String(), nil
}

// WriteDebugLog writes debug information to a log file
func WriteDebugLog(filename, content string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = file.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, content))
	return err
}
