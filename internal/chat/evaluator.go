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
	// Debug log the evaluation start
	WriteDebugLog("evaluation_debug.log", fmt.Sprintf("EVALUATION START:\nOriginal Request: %s\nHad Error: %t\nRemaining Commands: %v\nExecution Log: %s\n\n", originalRequest, hadError, remainingCommands, executionLog))

	// Step 1: Check if the original goal has been achieved
	goalAchieved, err := e.checkGoalAchievement(executionLog, originalRequest)
	if err != nil {
		WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Goal achievement check failed: %v\n", err))
		return nil, false, fmt.Errorf("failed to check goal achievement: %w", err)
	}

	if goalAchieved {
		WriteDebugLog("evaluation_debug.log", "Goal achieved! Stopping execution.\n\n")
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
	prompt.WriteString("\n\nConsider these guidelines:\n")
	prompt.WriteString("- For information requests (what/how/which/where questions), check if the command output contains the requested information\n")
	prompt.WriteString("- For time/date questions ('what time is it', 'what day is it'), ANY successful date command output provides the answer\n")
	prompt.WriteString("- For file/system modification requests, check if the intended changes were successfully made\n")
	prompt.WriteString("- If the command ran successfully and produced relevant output for an information request, the goal is achieved\n")
	prompt.WriteString("- Be liberal in recognizing success - if a single command provides the requested information, that's usually sufficient\n")
	prompt.WriteString("\nExamples of successful completion:\n")
	prompt.WriteString("- Request: 'what time is it?' + date command output → YES (time information was provided)\n")
	prompt.WriteString("- Request: 'what files are here?' + ls command output → YES (file listing was provided)\n")
	prompt.WriteString("\nIMPORTANT: You must respond with EXACTLY one word:\n")
	prompt.WriteString("- Type 'YES' if the goal has been achieved\n")
	prompt.WriteString("- Type 'NO' if more work is needed\n")
	prompt.WriteString("\nDo NOT explain your reasoning. Do NOT repeat the command. Just answer YES or NO.\n")
	prompt.WriteString("\nHas the original request been successfully completed? Answer: ")

	// Debug log the goal achievement evaluation
	WriteDebugLog("evaluation_debug.log", fmt.Sprintf("GOAL ACHIEVEMENT CHECK:\nPrompt: %s\n", prompt.String()))

	response, err := e.llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Goal achievement error: %v\n", err))
		return false, err
	}

	// Clean and parse the response more robustly
	cleanResponse := strings.TrimSpace(strings.ToUpper(response))
	
	// Check for various ways the AI might say yes
	result := cleanResponse == "YES" || 
	         cleanResponse == "Y" ||
	         strings.Contains(cleanResponse, "YES") ||
	         strings.Contains(cleanResponse, "ACHIEVED") ||
	         strings.Contains(cleanResponse, "COMPLETED") ||
	         strings.Contains(cleanResponse, "SUCCESS")
	         
	// Special case: if it's a time question and we have date output, assume success
	if strings.Contains(strings.ToLower(originalRequest), "time") && strings.Contains(executionLog, "$ date") && !strings.Contains(executionLog, "Error:") {
		result = true
		WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Goal achievement response: '%s' -> Overriding to true for time question with successful date command\n\n", response))
	} else {
		WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Goal achievement response: '%s' -> Result: %t\n\n", response, result))
	}

	return result, nil
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

// GenerateFinalAnswer creates a human-readable final answer based on the conversation
func (e *AIEvaluator) GenerateFinalAnswer(executionLog, originalRequest string) (string, error) {
	// Special handling for time questions with simple pattern matching
	if strings.Contains(strings.ToLower(originalRequest), "time") && strings.Contains(executionLog, "$ date") {
		// Extract the date output from the execution log
		lines := strings.Split(executionLog, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Look for a line that looks like date output (not starting with $)
			if line != "" && !strings.HasPrefix(line, "$") && !strings.HasPrefix(line, "#") {
				// Try to parse and reformat the time
				// Standard date output: "Sat Jul 12 00:10:49 EDT 2025"
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					// Try to extract time (3rd field usually) and format nicely
					timePart := parts[3] // "00:10:49"
					if strings.Contains(timePart, ":") {
						// Parse HH:MM:SS and convert to 12-hour format
						timeParts := strings.Split(timePart, ":")
						if len(timeParts) >= 2 {
							hour := timeParts[0]
							minute := timeParts[1]
							
							// Convert to 12-hour format
							var hourInt int
							if _, err := fmt.Sscanf(hour, "%d", &hourInt); err == nil {
								ampm := "AM"
								if hourInt >= 12 {
									ampm = "PM"
									if hourInt > 12 {
										hourInt -= 12
									}
								}
								if hourInt == 0 {
									hourInt = 12
								}
								return fmt.Sprintf("The current time is %d:%s %s.", hourInt, minute, ampm), nil
							}
						}
					}
				}
				// Fallback to showing the full date output
				return fmt.Sprintf("The current time is %s.", line), nil
			}
		}
	}
	
	// Special handling for IP address questions
	if strings.Contains(strings.ToLower(originalRequest), "ip") && (strings.Contains(executionLog, "$ ipconfig") || strings.Contains(executionLog, "$ ifconfig") || strings.Contains(executionLog, "$ curl")) {
		// Extract IP address from the execution log
		lines := strings.Split(executionLog, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Look for a line that looks like an IP address (not starting with $ and contains dots)
			if line != "" && !strings.HasPrefix(line, "$") && !strings.HasPrefix(line, "#") && strings.Contains(line, ".") {
				// Simple IP address pattern check (X.X.X.X)
				if strings.Count(line, ".") >= 3 {
					return fmt.Sprintf("Your IP address is %s.", line), nil
				}
			}
		}
	}

	var prompt strings.Builder
	prompt.WriteString("You are answering a user's question based on command output. DO NOT repeat commands or technical output.\n\n")
	prompt.WriteString("User asked: ")
	prompt.WriteString(originalRequest)
	prompt.WriteString("\n\nCommand output:\n")
	prompt.WriteString(executionLog)
	prompt.WriteString("\n\nIMPORTANT: You must provide a conversational answer in plain English. Do NOT just repeat the command name.\n")
	prompt.WriteString("Examples of good answers:\n")
	prompt.WriteString("- For 'what time is it?' with date output → 'The current time is 12:08 AM.'\n")
	prompt.WriteString("- For 'what files are here?' with ls output → 'There are 5 files: file1.txt, file2.py, etc.'\n")
	prompt.WriteString("\nYour answer (complete sentence, no commands): ")

	// Debug log the final answer generation
	WriteDebugLog("evaluation_debug.log", fmt.Sprintf("FINAL ANSWER GENERATION:\nPrompt: %s\n", prompt.String()))

	response, err := e.llmClient.GenerateResponse(prompt.String(), nil)
	if err != nil {
		WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Final answer generation error: %v\n", err))
		return "", err
	}

	finalAnswer := strings.TrimSpace(response)
	WriteDebugLog("evaluation_debug.log", fmt.Sprintf("Final answer response: '%s'\n\n", finalAnswer))

	return finalAnswer, nil
}
