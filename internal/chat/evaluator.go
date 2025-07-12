package chat

import (
	"fmt"
	"strings"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"
)

// AIEvaluator handles AI decision making for command execution
type AIEvaluator struct {
	llmClient        *llm.Client
	embeddingsClient *embeddings.Client
	vectorStore      *vector.ChromaClient
}

// NewAIEvaluator creates a new AI evaluator
func NewAIEvaluator(llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient) *AIEvaluator {
	return &AIEvaluator{
		llmClient:        llmClient,
		embeddingsClient: embeddingsClient,
		vectorStore:      vectorStore,
	}
}

// EvaluateAndGetNextCommands asks AI to evaluate command results using structured decision-making
func (e *AIEvaluator) EvaluateAndGetNextCommands(executionLog string, originalRequest string, remainingCommands []string, hadError bool) ([]string, bool, error) {
	// Step 1: Check if the original goal has been achieved
	goalAchieved, err := e.checkGoalAchievement(executionLog, originalRequest)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check goal achievement: %w", err)
	}

	if goalAchieved {
		return nil, false, nil
	}

	// Step 2: If goal not achieved, determine next steps based on current state
	if len(remainingCommands) == 0 {
		// Step 3: No commands queued - determine what to do next
		nextCommands, err := e.determineNextCommands(executionLog, originalRequest, hadError)
		if err != nil {
			return nil, false, fmt.Errorf("failed to determine next commands: %w", err)
		}
		return nextCommands, len(nextCommands) > 0, nil
	} else {
		// Step 4: Commands queued - decide whether to proceed or modify
		queueDecision, newCommands, err := e.evaluateCommandQueue(executionLog, originalRequest, remainingCommands, hadError)
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

// checkGoalAchievement determines if the original user request has been satisfied
func (e *AIEvaluator) checkGoalAchievement(executionLog, originalRequest string) (bool, error) {
	var prompt strings.Builder
	prompt.WriteString("Analyze whether the user's original request has been successfully completed.\n\n")
	prompt.WriteString("Original request: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nExecution log:\n")
	prompt.WriteString(executionLog)
	prompt.WriteString("\n\nHas the original request been successfully completed? ")
	prompt.WriteString("Respond with only 'YES' if the goal has been achieved, or 'NO' if more work is needed.")

	response, err := e.llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(strings.ToUpper(response)) == "YES", nil
}

// determineNextCommands decides what commands to execute next when none are queued
func (e *AIEvaluator) determineNextCommands(executionLog, originalRequest string, hadError bool) ([]string, error) {
	var prompt strings.Builder
	prompt.WriteString("You need to determine the next steps to achieve the user's goal.\n\n")
	prompt.WriteString("Original user request: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nCommand execution log:\n")
	prompt.WriteString(executionLog)
	
	if hadError {
		prompt.WriteString("\n\nThe last command failed. Analyze the error and determine alternative approaches.\n")
	} else {
		prompt.WriteString("\n\nThe previous commands succeeded. Determine what steps are needed next.\n")
	}
	
	prompt.WriteString("\nProvide the next commands to execute, one per line. ")
	prompt.WriteString("If no more commands are needed, respond with 'NONE'.")

	response, err := e.llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return nil, err
	}

	response = strings.TrimSpace(response)
	if response == "NONE" || response == "" {
		return nil, nil
	}

	validator := NewCommandValidator()
	return validator.ParseCommands(response), nil
}

// evaluateCommandQueue decides whether to proceed with planned commands or modify the plan
func (e *AIEvaluator) evaluateCommandQueue(executionLog string, originalRequest string, remainingCommands []string, hadError bool) (string, []string, error) {
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

	response, err := e.llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		return "", nil, err
	}

	response = strings.TrimSpace(response)
	lines := strings.Split(response, "\n")
	firstLine := strings.TrimSpace(strings.ToUpper(lines[0]))

	switch firstLine {
	case "PROCEED":
		return "proceed", nil, nil
	case "STOP":
		return "stop", nil, nil
	case "MODIFY":
		validator := NewCommandValidator()
		var newCommands []string
		for i := 1; i < len(lines); i++ {
			cmd := strings.TrimSpace(lines[i])
			if cmd != "" && !strings.HasPrefix(cmd, "#") && validator.IsValid(cmd) {
				newCommands = append(newCommands, cmd)
			}
		}
		return "modify", newCommands, nil
	default:
		return "stop", nil, nil
	}
}

// StoreExecutionSession stores the command execution session in ChromaDB for future learning
func (e *AIEvaluator) StoreExecutionSession(executionLog string) error {
	// Create a summary of the execution session
	summary := fmt.Sprintf("Command execution session:\n%s", executionLog)

	// Generate embedding for the execution session
	embedding, err := e.embeddingsClient.GenerateEmbedding(summary)
	if err != nil {
		return fmt.Errorf("failed to generate embedding for execution session: %w", err)
	}

	// Store in ChromaDB with a unique ID
	sessionID := fmt.Sprintf("cmd_session_%d", time.Now().Unix())
	if err := e.vectorStore.AddDocument(e.vectorStore.CommandsCollection(), sessionID, summary, embedding); err != nil {
		return fmt.Errorf("failed to store execution session: %w", err)
	}

	return nil
}
