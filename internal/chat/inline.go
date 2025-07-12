package chat

import (
	"fmt"
	"strings"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Simple inline model without viewport
type InlineModel struct {
	session         *Session
	textInput       textinput.Model
	spinner         spinner.Model
	state           string
	pendingCommand  string
	pendingExplanation string
	originalRequest string
	commandQueue    []string
	executionLog    strings.Builder
	currentAttempt  int
	quitting        bool
	
	// Styles
	userStyle     lipgloss.Style
	aiStyle       lipgloss.Style
	systemStyle   lipgloss.Style
	commandStyle  lipgloss.Style
	errorStyle    lipgloss.Style
}

func NewInlineSession(config *SessionConfig, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoIndexer *indexing.AutoIndexer) *InlineModel {
	session := NewSession(config, llmClient, embeddingsClient, vectorStore, autoIndexer)
	
	ti := textinput.New()
	ti.Placeholder = "Type your message and press Enter..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80
	
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	return &InlineModel{
		session:   session,
		textInput: ti,
		spinner:   s,
		state:     "input",
		userStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true),
		aiStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("120")),
		systemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		commandStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		errorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	}
}

func (m *InlineModel) Init() tea.Cmd {
	// Print welcome message
	fmt.Println(m.systemStyle.Render("🤖 RAG CLI Chat - Type 'help' for commands, Ctrl+C to quit"))
	if m.session.config.AutoApprove {
		fmt.Println(m.systemStyle.Render("⚡ Auto-approve is enabled"))
	}
	if m.session.config.AutoIndex {
		fmt.Println(m.systemStyle.Render("📂 Auto-indexing is enabled"))
	}
	fmt.Print("\n")
	
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *InlineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case "input":
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				input := strings.TrimSpace(m.textInput.Value())
				if input == "" {
					return m, nil
				}
				return m.handleInput(input)
			}
		case "approval":
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter", "y", "Y":
				return m.approveCommand()
			case "n", "N", "esc":
				return m.denyCommand()
			}
		}
		
	case aiResponseMsg:
		fmt.Print(m.aiStyle.Render("AI: ") + msg.response + "\n\n")
		if msg.err != nil {
			fmt.Println(m.errorStyle.Render(fmt.Sprintf("Error: %v", msg.err)))
			m.state = "input"
			return m, nil
		}
		
		// Check for commands
		validCommands := m.session.validator.ParseCommands(msg.response)
		if len(validCommands) > 0 {
			return m.handleCommands(validCommands)
		}
		
		m.state = "input"
		return m, nil
		
	case commandExecutedMsg:
		if msg.err != nil {
			fmt.Println(m.errorStyle.Render(fmt.Sprintf("❌ Command failed: %v", msg.err)))
			m.executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", msg.command, msg.err))
		} else {
			fmt.Println(m.commandStyle.Render(fmt.Sprintf("$ %s", msg.command)))
			if msg.output != "" {
				// Truncate output for display
				displayOutput := m.session.truncateOutputForDisplay(msg.output)
				fmt.Print(displayOutput)
				if !strings.HasSuffix(displayOutput, "\n") {
					fmt.Print("\n")
				}
			}
			fmt.Println(m.systemStyle.Render("✅ Command completed successfully"))
			m.executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", msg.command, msg.output))
			
			// Auto-index if enabled
			if m.session.autoIndexer != nil {
				go func() {
					if changedFiles, err := m.session.autoIndexer.DetectChanges(); err == nil && len(changedFiles) > 0 {
						if err := m.session.autoIndexer.IndexChangedFiles(changedFiles); err != nil {
							fmt.Println(m.systemStyle.Render(fmt.Sprintf("[Auto-index error: %v]", err)))
						}
					}
				}()
			}
		}
		fmt.Print("\n")
		return m.executeNextCommand()
		
	case nextCommandsMsg:
		if msg.err != nil {
			fmt.Println(m.errorStyle.Render(fmt.Sprintf("Evaluation error: %v", msg.err)))
			m.state = "input"
			return m, nil
		}
		
		if !msg.shouldContinue {
			// Generate final answer
			return m, tea.Cmd(func() tea.Msg {
				finalAnswer, err := m.session.evaluator.GenerateFinalAnswer(m.executionLog.String(), m.originalRequest)
				return finalAnswerMsg{answer: finalAnswer, err: err}
			})
		}
		
		// Continue with new commands
		m.currentAttempt++
		if m.currentAttempt > 1 {
			fmt.Println(m.systemStyle.Render(fmt.Sprintf("🔄 Retry attempt %d/%d", m.currentAttempt, m.session.config.MaxAttempts)))
		}
		m.commandQueue = msg.commands
		return m.executeNextCommand()
		
	case finalAnswerMsg:
		if msg.err != nil {
			fmt.Println(m.errorStyle.Render(fmt.Sprintf("Failed to generate final answer: %v", msg.err)))
		} else if msg.answer != "" {
			fmt.Print(m.aiStyle.Render("AI: ") + msg.answer + "\n\n")
		}
		fmt.Println(m.systemStyle.Render("✅ Task completed!"))
		fmt.Print("\n")
		m.state = "input"
		return m, nil
		
	case spinner.TickMsg:
		if m.state == "processing" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	
	// Update text input when in input state
	if m.state == "input" {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	
	return m, nil
}

func (m *InlineModel) View() string {
	if m.quitting {
		return m.systemStyle.Render("Goodbye!\n")
	}
	
	switch m.state {
	case "input":
		return fmt.Sprintf("%s %s", m.userStyle.Render(">"), m.textInput.View())
	case "processing":
		return fmt.Sprintf("%s Processing...", m.spinner.View())
	case "approval":
		content := fmt.Sprintf("⚠️  Command requires approval:\n")
		if m.pendingExplanation != "" {
			content += fmt.Sprintf("%s\n", m.pendingExplanation)
		}
		content += fmt.Sprintf("%s %s\n", m.commandStyle.Render("$"), m.pendingCommand)
		content += "Press Enter/Y to approve, N to deny: "
		return content
	}
	
	return ""
}

func (m *InlineModel) handleInput(input string) (tea.Model, tea.Cmd) {
	fmt.Printf("%s %s\n", m.userStyle.Render("You:"), input)
	
	// Handle special commands
	switch input {
	case "help", "?":
		m.showHelp()
		m.textInput.Reset()
		return m, nil
	case "clear":
		fmt.Print("\033[H\033[2J") // Clear screen
		fmt.Println(m.systemStyle.Render("🤖 RAG CLI Chat - Chat cleared"))
		fmt.Print("\n")
		m.textInput.Reset()
		return m, nil
	case "exit", "quit":
		m.quitting = true
		return m, tea.Quit
	}
	
	m.originalRequest = input
	m.textInput.Reset()
	m.state = "processing"
	
	return m, tea.Cmd(func() tea.Msg {
		context, err := m.session.contextManager.GetCombinedContext(input, !m.session.config.NoHistory, 5, 3)
		if err != nil {
			context = []string{}
		}
		
		response, err := m.session.llmClient.GenerateResponse(input, context)
		return aiResponseMsg{response: response, err: err}
	})
}

func (m *InlineModel) handleCommands(commands []string) (tea.Model, tea.Cmd) {
	m.commandQueue = commands
	m.currentAttempt = 1
	return m.executeNextCommand()
}

func (m *InlineModel) executeNextCommand() (tea.Model, tea.Cmd) {
	if len(m.commandQueue) == 0 {
		// Evaluate execution
		maxAttempts := m.session.config.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 3
		}
		
		if m.currentAttempt >= maxAttempts {
			fmt.Println(m.systemStyle.Render(fmt.Sprintf("❌ Max attempts (%d) reached", maxAttempts)))
			fmt.Print("\n")
			m.state = "input"
			return m, nil
		}
		
		return m, tea.Cmd(func() tea.Msg {
			nextCommands, shouldContinue, err := m.session.evaluator.EvaluateAndGetNextCommands(
				m.executionLog.String(),
				m.originalRequest,
				m.commandQueue,
				false,
			)
			return nextCommandsMsg{commands: nextCommands, shouldContinue: shouldContinue, err: err}
		})
	}
	
	command := m.commandQueue[0]
	m.commandQueue = m.commandQueue[1:]
	
	if !m.session.config.AutoApprove {
		explanation := m.session.generateCommandExplanation(command)
		m.pendingCommand = command
		m.pendingExplanation = explanation
		m.state = "approval"
		return m, nil
	} else {
		fmt.Println(m.systemStyle.Render(fmt.Sprintf("⚡ Auto-approving command: %s", command)))
		return m, tea.Cmd(func() tea.Msg {
			output, err := m.session.executor.Execute(command)
			return commandExecutedMsg{command: command, output: output, err: err}
		})
	}
}

func (m *InlineModel) approveCommand() (tea.Model, tea.Cmd) {
	command := m.pendingCommand
	m.pendingCommand = ""
	m.pendingExplanation = ""
	m.state = "processing"
	
	return m, tea.Cmd(func() tea.Msg {
		output, err := m.session.executor.Execute(command)
		return commandExecutedMsg{command: command, output: output, err: err}
	})
}

func (m *InlineModel) denyCommand() (tea.Model, tea.Cmd) {
	fmt.Println(m.systemStyle.Render("❌ Command execution cancelled by user"))
	fmt.Print("\n")
	m.pendingCommand = ""
	m.pendingExplanation = ""
	m.state = "input"
	return m, nil
}

func (m *InlineModel) showHelp() {
	help := `
RAG CLI Interactive Chat Help

Available commands:
  help, ?     - Show this help message
  clear       - Clear the screen
  exit, quit  - Exit the chat

Keyboard shortcuts:
  Enter       - Send message / Approve command (default)
  Y           - Approve command (when prompted)
  N           - Deny command (when prompted)
  Ctrl+C      - Exit chat

Features:
  • AI can execute shell commands with your approval
  • Auto-indexing of file changes (if enabled)
  • Context-aware responses using RAG
`
	fmt.Println(m.systemStyle.Render(help))
}

func (m *InlineModel) Run() error {
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
