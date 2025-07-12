package chat

// This test file focuses on testing the Y/n prompt response handling
// for AI-generated command execution approval.
//
// BACKGROUND:
// This RAG CLI system allows users to describe goals/tasks, and the AI
// generates appropriate shell commands to achieve those goals. Users can
// approve or deny each command via Y/n prompts.
//
// BUG THAT WAS FIXED:
// Previously, when a user answered 'n' to deny command execution, the system
// would only break out of the inner command loop but continue processing
// the AI response. The fix ensures that denial causes an immediate return
// to the chat prompt (interactive mode) or terminal prompt (non-interactive).
//
// These tests verify the core permission handling logic works correctly.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

import (
	"github.com/fatih/color"
)

// createTestSessionForPermissionTesting creates a minimal session for testing
// the permission handling logic without needing real LLM/vector clients
func createTestSessionForPermissionTesting(autoApprove bool) *Session {
	config := &SessionConfig{
		AutoApprove:     autoApprove,
		AutoIndex:       false,
		NoHistory:       true,
		MaxAttempts:     3, // Default for testing
		MaxOutputLines:  50, // Default for testing
		TruncateOutput:  true, // Default for testing
	}
	
	// Create a minimal session with just the config and colors for testing
	// We only need to test the permission logic, not the full AI functionality
	return &Session{
		config: config,
		// Initialize required color fields for testing - use simple colors
		commandColor: color.New(color.FgYellow),
		outputColor:  color.New(color.FgWhite),
		errorColor:   color.New(color.FgRed),
		infoColor:    color.New(color.FgBlue),
	}
}

// Helper function to capture stdin/stdout for testing user input
func withMockedInput(input string, fn func()) string {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	
	// Create a pipe for input
	r, w, _ := os.Pipe()
	os.Stdin = r
	
	// Write input to pipe
	go func() {
		defer w.Close()
		w.WriteString(input)
	}()
	
	// Capture stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r2)
		done <- true
	}()
	
	// Execute function
	fn()
	
	// Clean up
	w2.Close()
	<-done
	
	return output.String()
}

func TestRequestPermission_UserDenies(t *testing.T) {
	session := createTestSessionForPermissionTesting(false)
	
	// Test with "n" input
	output := withMockedInput("n\n", func() {
		result := session.requestPermission("echo test")
		if result != false {
			t.Errorf("Expected requestPermission to return false for 'n' input, got %v", result)
		}
	})
	
	if !strings.Contains(output, "Do you want to allow this? (Y/n):") {
		t.Errorf("Expected permission prompt in output, got: %s", output)
	}
}

func TestRequestPermission_UserApproves(t *testing.T) {
	session := createTestSessionForPermissionTesting(false)
	
	// Test with "y" input
	withMockedInput("y\n", func() {
		result := session.requestPermission("echo test")
		if result != true {
			t.Errorf("Expected requestPermission to return true for 'y' input, got %v", result)
		}
	})
	
	// Test with empty input (default yes)
	withMockedInput("\n", func() {
		result := session.requestPermission("echo test")
		if result != true {
			t.Errorf("Expected requestPermission to return true for empty input, got %v", result)
		}
	})
}

func TestRequestPermission_CaseInsensitive(t *testing.T) {
	session := createTestSessionForPermissionTesting(false)
	
	testCases := []struct {
		input    string
		expected bool
	}{
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"N\n", false},
		{"no\n", false},
		{"NO\n", false},
	}
	
	for _, tc := range testCases {
		withMockedInput(tc.input, func() {
			result := session.requestPermission("echo test")
			if result != tc.expected {
				t.Errorf("Expected requestPermission to return %v for input '%s', got %v", 
					tc.expected, strings.TrimSpace(tc.input), result)
			}
		})
	}
}

// TestRequestPermissionCancellation tests the core bug fix:
// When user answers 'n' to command execution, the permission function
// should return false, which should cause the calling code to exit early
func TestRequestPermissionCancellation(t *testing.T) {
	session := createTestSessionForPermissionTesting(false)
	
	// Test the core functionality: permission denial should return false
	// This is the key behavior that was fixed - denial should return false
	result := false
	withMockedInput("n\n", func() {
		result = session.requestPermission("echo 'test command'")
	})
	
	if result != false {
		t.Errorf("Expected requestPermission to return false when user denies, got %v", result)
	}
	
	// Test with "no" as well
	withMockedInput("no\n", func() {
		result = session.requestPermission("echo 'test command'")
	})
	
	if result != false {
		t.Errorf("Expected requestPermission to return false for 'no' input, got %v", result)
	}
}

// TestAutoApproveConfig verifies that the auto-approve configuration
// is properly stored in the session config
func TestAutoApproveConfig(t *testing.T) {
	session := createTestSessionForPermissionTesting(true) // Auto-approve enabled
	
	// Verify the config is set correctly
	if !session.config.AutoApprove {
		t.Errorf("Expected AutoApprove to be true, got %v", session.config.AutoApprove)
	}
	
	// Test with auto-approve disabled
	sessionNoAuto := createTestSessionForPermissionTesting(false)
	if sessionNoAuto.config.AutoApprove {
		t.Errorf("Expected AutoApprove to be false, got %v", sessionNoAuto.config.AutoApprove)
	}
}

// TestOutputTruncation tests the output truncation functionality
func TestOutputTruncation(t *testing.T) {
	// Test with truncation enabled
	session := createTestSessionForPermissionTesting(false)
	session.config.MaxOutputLines = 10
	session.config.TruncateOutput = true
	
	// Create output with many lines
	lines := make([]string, 20)
	for i := 0; i < 20; i++ {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	longOutput := strings.Join(lines, "\n")
	
	truncated := session.truncateOutputForDisplay(longOutput)
	
	// Should contain first 5 lines
	if !strings.Contains(truncated, "Line 1") || !strings.Contains(truncated, "Line 5") {
		t.Errorf("Expected truncated output to contain first 5 lines")
	}
	
	// Should contain last 5 lines
	if !strings.Contains(truncated, "Line 16") || !strings.Contains(truncated, "Line 20") {
		t.Errorf("Expected truncated output to contain last 5 lines")
	}
	
	// Should contain truncation indicator
	if !strings.Contains(truncated, "lines omitted") {
		t.Errorf("Expected truncated output to contain omission indicator")
	}
	
	// Should not contain middle lines
	if strings.Contains(truncated, "Line 10") {
		t.Errorf("Expected truncated output to not contain middle lines")
	}
}

func TestOutputTruncationDisabled(t *testing.T) {
	// Test with truncation disabled
	session := createTestSessionForPermissionTesting(false)
	session.config.TruncateOutput = false
	
	// Create output with many lines
	lines := make([]string, 20)
	for i := 0; i < 20; i++ {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	longOutput := strings.Join(lines, "\n")
	
	result := session.truncateOutputForDisplay(longOutput)
	
	// Should return the full output unchanged
	if result != longOutput {
		t.Errorf("Expected full output when truncation is disabled")
	}
}

func TestOutputTruncationShortOutput(t *testing.T) {
	// Test with short output that doesn't need truncation
	session := createTestSessionForPermissionTesting(false)
	session.config.MaxOutputLines = 10
	session.config.TruncateOutput = true
	
	shortOutput := "Line 1\nLine 2\nLine 3"
	result := session.truncateOutputForDisplay(shortOutput)
	
	// Should return unchanged
	if result != shortOutput {
		t.Errorf("Expected short output to remain unchanged")
	}
}

// NOTE: More complex integration tests involving executeCommandsIteratively
// would require mocking the executor, validator, and evaluator components.
// The core permission logic is tested above, and the integration behavior
// can be verified manually or with integration tests that use real components.
