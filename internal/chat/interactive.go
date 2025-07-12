package chat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

// InteractiveSession handles the interactive chat loop with readline
type InteractiveSession struct {
	session *Session
	rl      *readline.Instance
	
	// UI colors
	userPromptColor *color.Color
	aiResponseColor *color.Color
	separatorColor  *color.Color
	infoColor       *color.Color
	
	// Styling
	horizontalRule string
	lightRule      string
}

// NewInteractiveSession creates a new interactive chat session
func NewInteractiveSession(config *SessionConfig, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoIndexer *indexing.AutoIndexer) (*InteractiveSession, error) {
	session := NewSession(config, llmClient, embeddingsClient, vectorStore, autoIndexer)
	
	// Initialize UI colors
	userPromptColor := color.New(color.FgCyan, color.Bold)
	aiResponseColor := color.New(color.FgGreen)
	separatorColor := color.New(color.FgMagenta)
	infoColor := color.New(color.FgBlue)
	
	// Set up readline for interactive input
	rl, err := readline.NewEx(&readline.Config{
		Prompt:              userPromptColor.Sprintf("> "),
		HistoryFile:         filepath.Join(os.TempDir(), "ragcli_history.tmp"),
		InterruptPrompt:     "",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: func(r rune) (rune, bool) { return r, true },
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize readline: %w", err)
	}
	
	return &InteractiveSession{
		session:         session,
		rl:              rl,
		userPromptColor: userPromptColor,
		aiResponseColor: aiResponseColor,
		separatorColor:  separatorColor,
		infoColor:       infoColor,
		horizontalRule:  strings.Repeat("─", 60),
		lightRule:       strings.Repeat("·", 40),
	}, nil
}

// Close cleans up the interactive session
func (is *InteractiveSession) Close() {
	if is.rl != nil {
		is.rl.Close()
	}
}

// Run starts the interactive chat loop
func (is *InteractiveSession) Run() error {
	is.showWelcome()
	
	// Main interactive loop
	for {
		line, err := is.rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "exit" || input == "quit" {
			is.infoColor.Println("Goodbye!")
			break
		}

		if input == "" {
			continue
		}

		// Handle special commands
		if input == "help" || input == "?" {
			is.showHelp()
			continue
		}

		if input == "clear" {
			fmt.Print("\033[H\033[2J")
			continue
		}

		// Process the input with the AI
		is.handleInput(input)
	}
	
	return nil
}

// showWelcome displays the welcome message and current settings
func (is *InteractiveSession) showWelcome() {
	is.infoColor.Println("RAG CLI Chat - Type 'exit' to quit")
	is.separatorColor.Println(is.horizontalRule)
	
	// Show enabled features
	if is.session.config.AutoApprove {
		is.infoColor.Println("[Auto-approve enabled]")
	}
	if is.session.config.AutoIndex {
		is.infoColor.Println("[Auto-indexing enabled]")
	}
}

// showHelp displays help information for the interactive chat
func (is *InteractiveSession) showHelp() {
	is.separatorColor.Println(is.lightRule)
	is.infoColor.Println("RAG CLI Interactive Chat Help")
	is.separatorColor.Println(is.lightRule)
	fmt.Println("Available commands:")
	fmt.Println("  help, ?     - Show this help message")
	fmt.Println("  clear       - Clear the screen")
	fmt.Println("  exit, quit  - Exit the chat")
	fmt.Println("")
	fmt.Println("Features:")
	fmt.Println("  • Use ↑/↓ arrows to navigate command history")
	fmt.Println("  • Ctrl+A to jump to beginning of line")
	fmt.Println("  • Ctrl+E to jump to end of line")
	fmt.Println("  • Ctrl+C to cancel current input")
	fmt.Println("  • Ctrl+D to exit chat")
	fmt.Println("")
	fmt.Println("The AI can execute shell commands and will prompt for approval.")
	fmt.Println("Use --auto-approve flag to skip confirmation prompts.")
	is.separatorColor.Println(is.lightRule)
}

// handleInput processes user input and generates AI response
func (is *InteractiveSession) handleInput(input string) {
	// Get combined context
	context, err := is.session.contextManager.GetCombinedContext(input, !is.session.config.NoHistory, 5, 3)
	if err != nil {
		fmt.Printf("Warning: Failed to retrieve context: %v\n", err)
		context = []string{}
	}

	// Generate response using LLM
	response, err := is.session.llmClient.GenerateResponse(input, context)
	if err != nil {
		is.session.errorColor.Printf("Error generating response: %v\n", err)
		return
	}

	// Process response for commands and execute if needed
	enhancedResponse, err := is.session.processResponseWithCommands(response, input)
	if err != nil {
		is.session.errorColor.Printf("Error processing commands: %v\n", err)
		return
	}

	is.separatorColor.Println(is.horizontalRule)
	aiCmd := fmt.Sprintf("AI: %s", enhancedResponse)
	is.aiResponseColor.Println(aiCmd)
	is.separatorColor.Println(is.horizontalRule)
}
