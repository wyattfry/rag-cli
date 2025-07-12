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
	AllowCommands bool
	AutoApprove   bool
	AutoIndex     bool
	NoHistory     bool
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

	// Check if command execution is allowed
	if !s.config.AllowCommands {
		return response + "\n\n[Command execution is disabled. Use --allow-commands flag to enable.]", nil
	}

	// Ask user for permission to execute commands (unless auto-approved)
	if !s.config.AutoApprove {
		if !s.requestPermission(validCommands) {
			return response, nil
		}
	} else {
		s.infoColor.Printf("\nAuto-approving execution of %d command(s)...\n", len(validCommands))
	}

	// Execute commands iteratively with feedback
	return s.executeCommandsIteratively(validCommands, originalRequest)
}

// requestPermission asks the user for permission to execute commands
func (s *Session) requestPermission(commands []string) bool {
	s.infoColor.Printf("\nThe AI wants to execute the following command(s):\n")
	lightRule := strings.Repeat("·", 40)
	fmt.Println(lightRule)
	for _, cmd := range commands {
		s.commandColor.Printf("$ %s\n", cmd)
	}
	fmt.Println(lightRule)
	fmt.Printf("Do you want to allow this? (y/n): ")
	
	reader := bufio.NewReader(os.Stdin)
	permission, _ := reader.ReadString('\n')
	permission = strings.TrimSpace(strings.ToLower(permission))
	
	return permission == "y" || permission == "yes"
}

// executeCommandsIteratively executes commands one by one, allowing AI to refine approach based on results
func (s *Session) executeCommandsIteratively(initialCommands []string, originalRequest string) (string, error) {
	const maxAttempts = 3
	var executionLog strings.Builder
	var commandQueue []string

	// Start with initial commands
	commandQueue = append(commandQueue, initialCommands...)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts && len(commandQueue) > 0; attempt++ {
		if attempt > 1 {
			s.infoColor.Printf("\nAttempt %d/%d\n", attempt, maxAttempts)
		}

		// Execute all commands in the queue
		for len(commandQueue) > 0 {
			cmdStr := commandQueue[0]
			commandQueue = commandQueue[1:] // Remove executed command
			
			s.commandColor.Printf("\nExecuting: %s\n", cmdStr)
			
			output, err := s.executor.Execute(cmdStr)
			if err != nil {
				s.errorColor.Printf("Error: %v\n", err)
				// Include the actual command output (stderr) in the log for AI context
				if output != "" {
					executionLog.WriteString(fmt.Sprintf("$ %s\n%s\nError: %v\n\n", cmdStr, output, err))
				} else {
					executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", cmdStr, err))
				}
				lastErr = err
				break // Exit the current execution loop if there's an error
			} else {
				s.outputColor.Printf("%s", output)
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
			break
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
