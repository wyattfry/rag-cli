package chat

import (
	"strconv"
	"strings"
)

// CommandValidator handles validation of command strings
type CommandValidator struct{}

// NewCommandValidator creates a new command validator
func NewCommandValidator() *CommandValidator {
	return &CommandValidator{}
}

// IsValid checks if a command string is valid for execution
func (v *CommandValidator) IsValid(cmd string) bool {
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

// ParseCommands extracts valid commands from a response string
func (v *CommandValidator) ParseCommands(response string) []string {
	response = strings.TrimSpace(response)
	if response == "" {
		return []string{}
	}

	// Split into individual commands
	commands := strings.Split(response, "\n")
	var validCommands []string
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" && v.IsValid(cmd) {
			validCommands = append(validCommands, cmd)
		}
	}
	
	if len(validCommands) == 0 {
		return []string{}
	}
	return validCommands
}
