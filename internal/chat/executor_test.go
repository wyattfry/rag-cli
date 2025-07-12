package chat

import (
	"strings"
	"testing"
)

func TestCommandExecutor_Execute(t *testing.T) {
	executor := NewCommandExecutor()
	
	t.Run("simple successful command", func(t *testing.T) {
		output, err := executor.Execute("echo hello")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if strings.TrimSpace(output) != "hello" {
			t.Errorf("Expected 'hello', got: %q", strings.TrimSpace(output))
		}
	})
	
	t.Run("simple failed command", func(t *testing.T) {
		output, err := executor.Execute("nonexistentcommand12345")
		if err == nil {
			t.Fatal("Expected error for nonexistent command")
		}
		if !strings.Contains(err.Error(), "command failed") {
			t.Errorf("Expected 'command failed' in error, got: %v", err)
		}
		// Output should contain stderr information
		if !strings.Contains(output, "not found") && !strings.Contains(output, "command not found") {
			t.Errorf("Expected stderr info in output, got: %q", output)
		}
	})
	
	t.Run("successful piped command", func(t *testing.T) {
		output, err := executor.Execute("echo hello | wc -w")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		// wc -w should return "1" for one word
		if !strings.Contains(strings.TrimSpace(output), "1") {
			t.Errorf("Expected output to contain '1', got: %q", strings.TrimSpace(output))
		}
	})
	
	t.Run("piped command with first step failure", func(t *testing.T) {
		_, err := executor.Execute("nonexistentcommand12345 | wc -w")
		if err == nil {
			t.Fatal("Expected error for failed pipe")
		}
		if !strings.Contains(err.Error(), "command failed") {
			t.Errorf("Expected 'command failed' in error, got: %v", err)
		}
	})
	
	t.Run("piped command with second step failure", func(t *testing.T) {
		_, err := executor.Execute("echo hello | nonexistentcommand12345")
		if err == nil {
			t.Fatal("Expected error for failed pipe")
		}
		if !strings.Contains(err.Error(), "pipe step 2 failed") {
			t.Errorf("Expected 'pipe step 2 failed' in error, got: %v", err)
		}
	})
	
	t.Run("complex piped command", func(t *testing.T) {
		// This should work: echo three lines, take first 2, count lines
		output, err := executor.Execute("echo -e 'line1\\nline2\\nline3' | head -2 | wc -l")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		// Should output "2" (two lines)
		if !strings.Contains(strings.TrimSpace(output), "2") {
			t.Errorf("Expected output to contain '2', got: %q", strings.TrimSpace(output))
		}
	})
}

func TestCommandExecutor_ExecutePipedCommand(t *testing.T) {
	executor := NewCommandExecutor()
	
	t.Run("command without pipes falls back to normal execution", func(t *testing.T) {
		output, err := executor.executePipedCommand("echo hello")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if strings.TrimSpace(output) != "hello" {
			t.Errorf("Expected 'hello', got: %q", strings.TrimSpace(output))
		}
	})
	
	t.Run("empty pipe parts are skipped", func(t *testing.T) {
		// This has empty parts but should still work
		output, err := executor.executePipedCommand("echo hello |  | wc -w")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if !strings.Contains(strings.TrimSpace(output), "1") {
			t.Errorf("Expected output to contain '1', got: %q", strings.TrimSpace(output))
		}
	})
}

// Test helper to verify error messages contain expected information
func TestErrorMessageFormat(t *testing.T) {
	executor := NewCommandExecutor()
	
	t.Run("first step error includes stderr", func(t *testing.T) {
		output, err := executor.Execute("ls /nonexistenttestdir123456")
		if err == nil {
			t.Fatal("Expected error for nonexistent directory")
		}
		
		// Check that output contains helpful error message
		if !strings.Contains(output, "No such file or directory") &&
		   !strings.Contains(output, "cannot access") &&
		   !strings.Contains(output, "not found") {
			t.Errorf("Expected helpful error message in output, got: %q", output)
		}
	})
	
	t.Run("pipe step error identifies which step failed", func(t *testing.T) {
		_, err := executor.Execute("echo hello | invalidcommand123 | wc -l")
		if err == nil {
			t.Fatal("Expected error for invalid command in pipe")
		}
		
		if !strings.Contains(err.Error(), "pipe step") {
			t.Errorf("Expected 'pipe step' in error message, got: %v", err)
		}
	})
}
