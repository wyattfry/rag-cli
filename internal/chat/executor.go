package chat

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CommandExecutor handles the execution of shell commands with proper pipe handling
type CommandExecutor struct{}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{}
}

// Execute runs a shell command and returns its output
// If the command contains pipes, it splits and executes each part separately
// to provide better visibility into intermediate outputs
func (e *CommandExecutor) Execute(cmdStr string) (string, error) {
	// Check if command contains pipes
	if strings.Contains(cmdStr, " | ") {
		return e.executePipedCommand(cmdStr)
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
func (e *CommandExecutor) executePipedCommand(cmdStr string) (string, error) {
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
