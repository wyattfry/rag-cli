package chat

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"

	"github.com/charmbracelet/lipgloss"
)

type SimpleSession struct {
	session         *Session
	originalRequest string
	commandQueue    []string
	executionLog    strings.Builder
	currentAttempt  int
	
	// Styles
	userStyle     lipgloss.Style
	aiStyle       lipgloss.Style
	systemStyle   lipgloss.Style
	commandStyle  lipgloss.Style
	errorStyle    lipgloss.Style
	promptStyle   lipgloss.Style
}

func NewSimpleSession(config *SessionConfig, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoIndexer *indexing.AutoIndexer) *SimpleSession {
	session := NewSession(config, llmClient, embeddingsClient, vectorStore, autoIndexer)
	
	return &SimpleSession{
		session:     session,
		userStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true),
		aiStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("120")),
		systemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		commandStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		errorStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		promptStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true),
	}
}

func (s *SimpleSession) Run() error {
	// Print welcome message
	fmt.Println(s.systemStyle.Render("🤖 RAG CLI Chat - Type 'help' for commands, Ctrl+C to quit"))
	if s.session.config.AutoApprove {
		fmt.Println(s.systemStyle.Render("⚡ Auto-approve is enabled"))
	}
	if s.session.config.AutoIndex {
		fmt.Println(s.systemStyle.Render("📂 Auto-indexing is enabled"))
	}
	fmt.Println()
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		// Show prompt
		fmt.Print(s.promptStyle.Render("> "))
		
		// Read input
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		
		// Handle special commands
		if s.handleSpecialCommands(input) {
			continue
		}
		
		// Process with AI (don't reprint the input, user already sees it)
		if err := s.handleUserInput(input); err != nil {
			fmt.Println(s.errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		}
		
		fmt.Println() // Add spacing between interactions
	}
}

func (s *SimpleSession) handleSpecialCommands(input string) bool {
	switch input {
	case "help", "?":
		s.showHelp()
		return true
	case "clear":
		fmt.Print("\033[H\033[2J") // Clear screen
		fmt.Println(s.systemStyle.Render("🤖 RAG CLI Chat - Chat cleared"))
		fmt.Println()
		return true
	case "exit", "quit":
		fmt.Println(s.systemStyle.Render("Goodbye!"))
		os.Exit(0)
	}
	return false
}

func (s *SimpleSession) handleUserInput(input string) error {
	s.originalRequest = input
	
	// Get context
	context, err := s.session.contextManager.GetCombinedContext(input, !s.session.config.NoHistory, 5, 3)
	if err != nil {
		context = []string{}
	}
	
	// Generate response
	response, err := s.session.llmClient.GenerateResponse(input, context)
	if err != nil {
		return err
	}
	
	// Check for commands first
	validCommands := s.session.validator.ParseCommands(response)
	if len(validCommands) > 0 {
		// If response contains only commands, don't show the raw command text
		if strings.TrimSpace(response) != validCommands[0] || len(validCommands) > 1 {
			// Show AI response if it's more than just a bare command
			fmt.Printf("%s %s\n", s.aiStyle.Render("AI:"), response)
		}
		return s.executeCommandsIteratively(validCommands)
	}
	
	// Show AI response for non-command responses
	fmt.Printf("%s %s\n", s.aiStyle.Render("AI:"), response)
	
	return nil
}

func (s *SimpleSession) executeCommandsIteratively(initialCommands []string) error {
	maxAttempts := s.session.config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	
	s.commandQueue = initialCommands
	s.currentAttempt = 1
	s.executionLog.Reset()
	
	reader := bufio.NewReader(os.Stdin)
	var lastErr error
	
	for s.currentAttempt <= maxAttempts && len(s.commandQueue) > 0 {
		// Show attempt number if retrying
		if s.currentAttempt > 1 {
			fmt.Println(s.systemStyle.Render(fmt.Sprintf("🔄 Retry attempt %d/%d", s.currentAttempt, maxAttempts)))
			if lastErr != nil {
				fmt.Println(s.systemStyle.Render("❌ That didn't seem to work, let me try something else..."))
			} else {
				fmt.Println(s.systemStyle.Render("✅ That seemed to work, moving on to the next planned command..."))
			}
		}
		
		// Execute all commands in the queue
		lastErr = nil
		for len(s.commandQueue) > 0 {
			command := s.commandQueue[0]
			s.commandQueue = s.commandQueue[1:]
			
			// Ask for permission (unless auto-approved)
			if !s.session.config.AutoApprove {
				if !s.requestPermission(command, reader) {
					fmt.Println(s.systemStyle.Render("❌ Command execution cancelled by user"))
					return nil
				}
			} else {
				fmt.Println(s.systemStyle.Render(fmt.Sprintf("⚡ Auto-approving command: %s", command)))
			}
			
			// Execute command
			fmt.Println(s.commandStyle.Render(fmt.Sprintf("$ %s", command)))
			output, err := s.session.executor.Execute(command)
			
			if err != nil {
				fmt.Println(s.errorStyle.Render(fmt.Sprintf("❌ Command failed: %v", err)))
				// Include the actual command output (stderr) in the log for AI context
				if output != "" {
					s.executionLog.WriteString(fmt.Sprintf("$ %s\n%s\nError: %v\n\n", command, output, err))
				} else {
					s.executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", command, err))
				}
				lastErr = err
				break // Exit the current execution loop if there's an error
			} else {
				// Show output
				if output != "" {
					displayOutput := s.session.truncateOutputForDisplay(output)
					fmt.Print(displayOutput)
					if !strings.HasSuffix(displayOutput, "\n") {
						fmt.Print("\n")
					}
				}
				fmt.Println(s.systemStyle.Render("✅ Command completed successfully"))
				
				// Store full output in execution log for AI processing
				s.executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", command, output))
				lastErr = nil
				
				// Auto-index if enabled
				if s.session.autoIndexer != nil {
					go func() {
						if changedFiles, err := s.session.autoIndexer.DetectChanges(); err == nil && len(changedFiles) > 0 {
							if err := s.session.autoIndexer.IndexChangedFiles(changedFiles); err != nil {
								fmt.Println(s.systemStyle.Render(fmt.Sprintf("[Auto-index error: %v]", err)))
							}
						}
					}()
				}
			}
		}
		
		// Evaluate results and get new commands if needed
		nextCommands, shouldContinue, evalErr := s.session.evaluator.EvaluateAndGetNextCommands(
			s.executionLog.String(),
			s.originalRequest,
			s.commandQueue,
			lastErr != nil,
		)
		
		if evalErr != nil {
			fmt.Printf("Error evaluating results: %v\n", evalErr)
			break
		}
		
		if !shouldContinue {
			// Generate a final human-readable answer when goal is achieved
			finalAnswer, err := s.session.evaluator.GenerateFinalAnswer(s.executionLog.String(), s.originalRequest)
			if err == nil && finalAnswer != "" {
				fmt.Printf("%s %s\n", s.aiStyle.Render("AI:"), finalAnswer)
			} else if err != nil {
				fmt.Println(s.systemStyle.Render(fmt.Sprintf("Warning: Failed to generate final answer: %v", err)))
			}
			fmt.Println(s.systemStyle.Render("✅ Task completed successfully!"))
			break
		}
		
		// Provide feedback about what happened and what's next
		if len(nextCommands) > 0 && s.currentAttempt > 1 {
			if lastErr != nil {
				fmt.Println(s.systemStyle.Render("❌ That didn't seem to work, let me try something else..."))
			} else {
				fmt.Println(s.systemStyle.Render("✅ That seemed to work, moving on to the next planned command..."))
			}
		}
		
		// Replace command queue with new commands
		s.commandQueue = nextCommands
		s.currentAttempt++
		
		// Show AI's decision to modify commands
		if len(nextCommands) > 0 && s.currentAttempt > 2 {
			fmt.Print(s.systemStyle.Render("AI suggests next command(s): "))
			for i, cmd := range nextCommands {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(s.commandStyle.Render(cmd))
			}
			fmt.Println()
		}
	}
	
	if len(s.commandQueue) > 0 {
		fmt.Println(s.systemStyle.Render(fmt.Sprintf("❌ Max attempts (%d) reached. Remaining commands not executed.", maxAttempts)))
	}
	
	// Store the execution session in ChromaDB for future learning
	if err := s.session.evaluator.StoreExecutionSession(s.executionLog.String()); err != nil {
		fmt.Println(s.systemStyle.Render(fmt.Sprintf("Warning: Failed to store execution session: %v", err)))
	}
	
	return nil
}

func (s *SimpleSession) requestPermission(command string, reader *bufio.Reader) bool {
	// Generate explanation
	explanation := s.session.generateCommandExplanation(command)
	if explanation != "" {
		fmt.Println(s.systemStyle.Render(explanation))
	}
	
	fmt.Println(s.commandStyle.Render(fmt.Sprintf("$ %s", command)))
	fmt.Print("Press Enter/Y to approve, N to deny: ")
	
	permission, _ := reader.ReadString('\n')
	permission = strings.TrimSpace(strings.ToLower(permission))
	
	// Default to yes if user just presses Enter (empty string)
	// Only deny if user explicitly types "n" or "no"
	return permission == "" || permission == "y" || permission == "yes"
}

func (s *SimpleSession) showHelp() {
	help := `
RAG CLI Interactive Chat Help

Available commands:
  help, ?     - Show this help message
  clear       - Clear the screen
  exit, quit  - Exit the chat

Usage:
  • Type your message and press Enter
  • AI can execute shell commands with your approval
  • Press Enter or Y to approve commands (Enter is default)
  • Press N to deny command execution
  • Auto-indexing of file changes (if enabled)
  • Context-aware responses using RAG

Examples:
  "What files are in this directory?"
  "Show me the git status"
  "Create a new file called test.txt"
`
	fmt.Println(s.systemStyle.Render(help))
}
