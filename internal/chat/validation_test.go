package chat

import (
	"reflect"
	"testing"
)

func TestCommandValidator_IsValid(t *testing.T) {
	validator := NewCommandValidator()
	
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "valid simple command",
			command:  "ls -la",
			expected: true,
		},
		{
			name:     "valid command with pipe",
			command:  "ps aux | grep chrome",
			expected: true,
		},
		{
			name:     "empty command",
			command:  "",
			expected: false,
		},
		{
			name:     "command with shell prompt",
			command:  "$ ls -la",
			expected: false,
		},
		{
			name:     "command with hash prompt",
			command:  "# this is a comment",
			expected: false,
		},
		{
			name:     "command with greater than prompt",
			command:  "> output.txt",
			expected: false,
		},
		{
			name:     "command with newline",
			command:  "ls\ngrep something",
			expected: false,
		},
		{
			name:     "directory listing output",
			command:  "drwxr-xr-x 5 user group 160 Jan 1 12:00 folder",
			expected: false,
		},
		{
			name:     "total line from ls",
			command:  "total 64",
			expected: false,
		},
		{
			name:     "pure number",
			command:  "42",
			expected: false,
		},
		{
			name:     "error message",
			command:  "Error: command not found",
			expected: false,
		},
		{
			name:     "command not found error",
			command:  "bash: foo: command not found",
			expected: false,
		},
		{
			name:     "valid complex command",
			command:  "find /path -name '*.go' -exec grep -l 'package' {} \\;",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.IsValid(tt.command)
			if result != tt.expected {
				t.Errorf("IsValid(%q) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestCommandValidator_ParseCommands(t *testing.T) {
	validator := NewCommandValidator()
	
	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name:     "single valid command",
			response: "ls -la",
			expected: []string{"ls -la"},
		},
		{
			name: "multiple valid commands",
			response: `ls -la
grep something file.txt
echo "hello world"`,
			expected: []string{"ls -la", "grep something file.txt", "echo \"hello world\""},
		},
		{
			name: "mixed valid and invalid commands",
			response: `ls -la
$ invalid shell prompt
grep something file.txt
42
echo "hello world"
Error: some error`,
			expected: []string{"ls -la", "grep something file.txt", "echo \"hello world\""},
		},
		{
			name:     "empty response",
			response: "",
			expected: []string{},
		},
		{
			name: "only invalid commands",
			response: `$ shell prompt
# comment
42
Error: failed`,
			expected: []string{},
		},
		{
			name: "commands with extra whitespace",
			response: `   ls -la   
			
   grep something   
			`,
			expected: []string{"ls -la", "grep something"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ParseCommands(tt.response)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseCommands(%q) = %v, expected %v", tt.response, result, tt.expected)
			}
		})
	}
}
