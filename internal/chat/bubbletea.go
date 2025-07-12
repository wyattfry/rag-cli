package chat

import (
	"fmt"
	"strings"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/indexing"
	"rag-cli/internal/llm"
	"rag-cli/internal/vector"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// States for the application
type state int

const (
	stateInput state = iota
	stateProcessing
	stateWaitingApproval
	stateError
)

// Message types for Bubble Tea
type aiResponseMsg struct {
	response string
	err      error
}

type commandExecutedMsg struct {
	command string
	output  string
	err     error
}

type commandApprovalMsg struct {
	command     string
	explanation string
}

type nextCommandsMsg struct {
	commands      []string
	shouldContinue bool
	err           error
}

type finalAnswerMsg struct {
	answer string
	err    error
}

type Model struct {
	// Core session components
	session *Session
	
	// UI state
	state        state
	width        int
	height       int
	
	// Bubble Tea components
	textarea     textarea.Model
	viewport     viewport.Model
	spinner      spinner.Model
	
	// Chat history
	messages     []ChatMessage
	
	// Current command awaiting approval
	pendingCommand string
	pendingExplanation string
	
	// Iterative execution state
	commandQueue    []string
	originalRequest string
	executionLog    strings.Builder
	currentAttempt  int
	
	// Styles
	styles       Styles
	
	// Input handling
	ready        bool
	err          error
}

type ChatMessage struct {
	Type      string    // "user", "ai", "system", "command", "output", "error"
	Content   string
	Timestamp time.Time
}

type Styles struct {
	Base          lipgloss.Style
	Header        lipgloss.Style
	UserMessage   lipgloss.Style
	AIMessage     lipgloss.Style
	SystemMessage lipgloss.Style
	CommandStyle  lipgloss.Style
	OutputStyle   lipgloss.Style
	ErrorStyle    lipgloss.Style
	StatusBar     lipgloss.Style
	InputBox      lipgloss.Style
	Spinner       lipgloss.Style
	Approval      lipgloss.Style
}

func NewStyles() Styles {
	return Styles{
		Base: lipgloss.NewStyle().
			Padding(1, 2),
		
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(0, 1).
			Margin(1, 0),
		
		UserMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")).
			Bold(true).
			MarginLeft(2).
			MarginBottom(1),
		
		AIMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			MarginLeft(2).
			MarginBottom(1),
		
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			MarginLeft(2).
			MarginBottom(1),
		
		CommandStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1).
			MarginLeft(2).
			MarginBottom(1),
		
		OutputStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Background(lipgloss.Color("#111827")).
			Padding(0, 1).
			MarginLeft(4).
			MarginBottom(1),
		
		ErrorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true).
			MarginLeft(2).
			MarginBottom(1),
		
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1),
		
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#06B6D4")).
			Padding(0, 1),
		
		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")),
		
		Approval: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Background(lipgloss.Color("#1F2937")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(1, 2).
			Margin(1, 0),
	}
}

func NewBubbleTeaSession(config *SessionConfig, llmClient *llm.Client, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, autoIndexer *indexing.AutoIndexer) *Model {
	session := NewSession(config, llmClient, embeddingsClient, vectorStore, autoIndexer)
	
	// Initialize textarea
	ti := textarea.New()
	ti.Placeholder = "Type your message here... (Ctrl+C to quit, Tab to send)"
	ti.Focus()
	ti.CharLimit = 2000
	ti.SetWidth(80)
	ti.SetHeight(3)
	ti.ShowLineNumbers = false
	ti.KeyMap.InsertNewline.SetEnabled(false) // Disable newlines in textarea
	
	// Initialize viewport for chat history
	vp := viewport.New(80, 20)
	vp.SetContent("")
	
	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	
	m := &Model{
		session:   session,
		state:     stateInput,
		textarea:  ti,
		viewport:  vp,
		spinner:   s,
		messages:  []ChatMessage{},
		styles:    NewStyles(),
		ready:     false,
	}
	
	// Add welcome message
	m.addSystemMessage("🤖 RAG CLI Chat - Welcome! Type your questions or commands.")
	if config.AutoApprove {
		m.addSystemMessage("⚡ Auto-approve is enabled")
	}
	if config.AutoIndex {
		m.addSystemMessage("📂 Auto-indexing is enabled")
	}
	
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.textarea.Focus(),
		m.spinner.Tick,
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Adjust viewport size
		headerHeight := 5
		statusHeight := 2
		inputHeight := 5
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - headerHeight - statusHeight - inputHeight
		
		// Adjust textarea size
		m.textarea.SetWidth(msg.Width - 4)
		
		if !m.ready {
			m.ready = true
		}
		
		return m, nil
	
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.state == stateInput && strings.TrimSpace(m.textarea.Value()) != "" {
				return m.sendMessage()
			}
		case "enter":
			if m.state == stateWaitingApproval {
				// Enter defaults to approve (Y)
				return m.approveCommand()
			}
		case "y", "Y":
			if m.state == stateWaitingApproval {
				return m.approveCommand()
			}
		case "n", "N":
			if m.state == stateWaitingApproval {
				return m.denyCommand()
			}
		case "esc":
			if m.state == stateWaitingApproval {
				return m.denyCommand()
			}
		}
	
	case aiResponseMsg:
		if msg.err != nil {
			m.addErrorMessage(fmt.Sprintf("Error: %v", msg.err))
			m.state = stateInput
		} else {
			m.addAIMessage(msg.response)
			// Check if the response contains commands that need approval
			validCommands := m.session.validator.ParseCommands(msg.response)
			if len(validCommands) > 0 && !m.session.config.AutoApprove {
				// Show first command for approval
				command := validCommands[0]
				explanation := m.session.generateCommandExplanation(command)
				m.pendingCommand = command
				m.pendingExplanation = explanation
				m.state = stateWaitingApproval
				return m, nil
			} else if len(validCommands) > 0 {
				// Auto-approve enabled, execute commands
				return m.executeCommands(validCommands)
			} else {
				m.state = stateInput
			}
		}
		m.updateViewport()
	
	case commandExecutedMsg:
		if msg.err != nil {
			m.addErrorMessage(fmt.Sprintf("Command failed: %v", msg.err))
			// Log the failed command
			if msg.output != "" {
				m.executionLog.WriteString(fmt.Sprintf("$ %s\n%s\nError: %v\n\n", msg.command, msg.output, msg.err))
			} else {
				m.executionLog.WriteString(fmt.Sprintf("$ %s\nError: %v\n\n", msg.command, msg.err))
			}
		} else {
			m.addCommandMessage(msg.command)
			if msg.output != "" {
				m.addOutputMessage(msg.output)
			}
			m.addSystemMessage("✅ Command completed successfully")
			// Log the successful command
			m.executionLog.WriteString(fmt.Sprintf("$ %s\n%s\n\n", msg.command, msg.output))
			
			// Auto-index if enabled
			if m.session.autoIndexer != nil {
				go func() {
					if changedFiles, err := m.session.autoIndexer.DetectChanges(); err == nil && len(changedFiles) > 0 {
						if err := m.session.autoIndexer.IndexChangedFiles(changedFiles); err != nil {
							m.addSystemMessage(fmt.Sprintf("[Auto-index error: %v]", err))
						}
					}
				}()
			}
		}
		m.updateViewport()
		// Continue with next command or evaluation
		return m.executeNextCommand()
	
	case nextCommandsMsg:
		if msg.err != nil {
			m.addErrorMessage(fmt.Sprintf("Evaluation error: %v", msg.err))
			m.state = stateInput
		} else if !msg.shouldContinue {
			// Task completed, generate final answer
			m.state = stateProcessing
			return m, tea.Cmd(func() tea.Msg {
				finalAnswer, err := m.session.evaluator.GenerateFinalAnswer(m.executionLog.String(), m.originalRequest)
				return finalAnswerMsg{answer: finalAnswer, err: err}
			})
		} else {
			// Continue with new commands
			m.currentAttempt++
			if m.currentAttempt > 1 {
				m.addSystemMessage(fmt.Sprintf("🔄 Retry attempt %d/%d", m.currentAttempt, m.session.config.MaxAttempts))
			}
			m.commandQueue = msg.commands
			return m.executeNextCommand()
		}
		m.updateViewport()
	
	case finalAnswerMsg:
		if msg.err != nil {
			m.addErrorMessage(fmt.Sprintf("Failed to generate final answer: %v", msg.err))
		} else if msg.answer != "" {
			m.addAIMessage(msg.answer)
			m.addSystemMessage("✅ Task completed successfully!")
		} else {
			m.addSystemMessage("✅ Task completed!")
		}
		m.state = stateInput
		m.updateViewport()
	
	case spinner.TickMsg:
		if m.state == stateProcessing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	
	// Update components based on current state
	switch m.state {
	case stateInput:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	case stateProcessing:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	// Always update viewport for scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	
	var sections []string
	
	// Header
	header := m.styles.Header.Render("🤖 RAG CLI Interactive Chat")
	sections = append(sections, header)
	
	// Chat viewport
	chatView := m.styles.Base.Render(m.viewport.View())
	sections = append(sections, chatView)
	
	// Status bar
	var status string
	switch m.state {
	case stateInput:
		status = "Ready - Type your message and press Tab to send"
	case stateProcessing:
		status = fmt.Sprintf("%s Processing your request...", m.spinner.View())
	case stateWaitingApproval:
		status = "⚠️  Command approval required - Press Enter/Y to approve, N to deny"
	case stateError:
		status = "❌ Error occurred"
	}
	statusBar := m.styles.StatusBar.Width(m.width).Render(status)
	sections = append(sections, statusBar)
	
	// Command approval box (if waiting for approval)
	if m.state == stateWaitingApproval {
		approvalContent := fmt.Sprintf("Command requires approval:\n\n$ %s", m.pendingCommand)
		if m.pendingExplanation != "" {
			approvalContent = fmt.Sprintf("%s\n\n%s", m.pendingExplanation, approvalContent)
		}
		approvalContent += "\n\nPress Enter/Y to approve, N to deny"
		approval := m.styles.Approval.Width(m.width-4).Render(approvalContent)
		sections = append(sections, approval)
	} else {
		// Input box (only show when not waiting for approval)
		inputBox := m.styles.InputBox.Render(m.textarea.View())
		sections = append(sections, inputBox)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) sendMessage() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}
	
	// Handle special commands
	switch input {
	case "help", "?":
		m.showHelp()
		m.textarea.Reset()
		m.updateViewport()
		return m, nil
	case "clear":
		m.messages = []ChatMessage{}
		m.addSystemMessage("🤖 RAG CLI Chat - Chat cleared")
		m.textarea.Reset()
		m.updateViewport()
		return m, nil
	case "exit", "quit":
		return m, tea.Quit
	}
	
	// Add user message
	m.addUserMessage(input)
	m.originalRequest = input // Store for iterative execution
	m.textarea.Reset()
	m.state = stateProcessing
	m.updateViewport()
	
	// Process with AI
	return m, tea.Cmd(func() tea.Msg {
		// Get context
		context, err := m.session.contextManager.GetCombinedContext(input, !m.session.config.NoHistory, 5, 3)
		if err != nil {
			context = []string{}
		}
		
		// Generate response
		response, err := m.session.llmClient.GenerateResponse(input, context)
		return aiResponseMsg{response: response, err: err}
	})
}

func (m *Model) approveCommand() (tea.Model, tea.Cmd) {
	if m.pendingCommand == "" {
		m.state = stateInput
		return m, nil
	}
	
	command := m.pendingCommand
	m.pendingCommand = ""
	m.pendingExplanation = ""
	m.state = stateProcessing
	
	return m, tea.Cmd(func() tea.Msg {
		output, err := m.session.executor.Execute(command)
		return commandExecutedMsg{command: command, output: output, err: err}
	})
}

func (m *Model) denyCommand() (tea.Model, tea.Cmd) {
	m.addSystemMessage("❌ Command execution cancelled by user")
	m.pendingCommand = ""
	m.pendingExplanation = ""
	m.state = stateInput
	m.updateViewport()
	return m, nil
}

func (m *Model) executeCommands(commands []string) (tea.Model, tea.Cmd) {
	if len(commands) == 0 {
		m.state = stateInput
		return m, nil
	}
	
	// Initialize or update command queue
	m.commandQueue = commands
	m.currentAttempt = 1
	
	// Execute the first command
	return m.executeNextCommand()
}

func (m *Model) executeNextCommand() (tea.Model, tea.Cmd) {
	if len(m.commandQueue) == 0 {
		// No more commands, evaluate if we should continue
		return m.evaluateExecution()
	}
	
	// Get the next command
	command := m.commandQueue[0]
	m.commandQueue = m.commandQueue[1:]
	
	if !m.session.config.AutoApprove {
		// Need approval for this command
		explanation := m.session.generateCommandExplanation(command)
		m.pendingCommand = command
		m.pendingExplanation = explanation
		m.state = stateWaitingApproval
		return m, nil
	} else {
		// Auto-approve, execute immediately
		m.addSystemMessage(fmt.Sprintf("⚡ Auto-approving command: %s", command))
		m.state = stateProcessing
		return m, tea.Cmd(func() tea.Msg {
			output, err := m.session.executor.Execute(command)
			return commandExecutedMsg{command: command, output: output, err: err}
		})
	}
}

func (m *Model) evaluateExecution() (tea.Model, tea.Cmd) {
	maxAttempts := m.session.config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	
	if m.currentAttempt >= maxAttempts {
		m.addSystemMessage(fmt.Sprintf("❌ Max attempts (%d) reached", maxAttempts))
		m.state = stateInput
		return m, nil
	}
	
	m.state = stateProcessing
	return m, tea.Cmd(func() tea.Msg {
		// Evaluate results and get next commands
		nextCommands, shouldContinue, err := m.session.evaluator.EvaluateAndGetNextCommands(
			m.executionLog.String(),
			m.originalRequest,
			m.commandQueue,
			false, // TODO: track if last command failed
		)
		return nextCommandsMsg{commands: nextCommands, shouldContinue: shouldContinue, err: err}
	})
}

func (m *Model) addUserMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Type:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) addAIMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Type:      "ai",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) addSystemMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Type:      "system",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) addCommandMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Type:      "command",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) addOutputMessage(content string) {
	// Truncate long output for display
	displayContent := m.session.truncateOutputForDisplay(content)
	m.messages = append(m.messages, ChatMessage{
		Type:      "output",
		Content:   displayContent,
		Timestamp: time.Now(),
	})
}

func (m *Model) addErrorMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Type:      "error",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) showHelp() {
	helpText := `RAG CLI Interactive Chat Help

Available commands:
  help, ?     - Show this help message
  clear       - Clear the chat history
  exit, quit  - Exit the chat

Keyboard shortcuts:
  Tab         - Send message
  Ctrl+C      - Exit chat
  Enter/Y     - Approve command (when prompted)
  N           - Deny command (when prompted)
  Esc         - Deny command (when prompted)

Features:
  • AI can execute shell commands with your approval
  • Rich formatting and syntax highlighting
  • Auto-indexing of file changes (if enabled)
  • Context-aware responses using RAG`
	
	m.addSystemMessage(helpText)
}

func (m *Model) updateViewport() {
	var content strings.Builder
	
	for _, msg := range m.messages {
		var rendered string
		timestamp := msg.Timestamp.Format("15:04:05")
		
		switch msg.Type {
		case "user":
			rendered = m.styles.UserMessage.Render(fmt.Sprintf("[%s] You: %s", timestamp, msg.Content))
		case "ai":
			rendered = m.styles.AIMessage.Render(fmt.Sprintf("[%s] AI: %s", timestamp, msg.Content))
		case "system":
			rendered = m.styles.SystemMessage.Render(fmt.Sprintf("[%s] %s", timestamp, msg.Content))
		case "command":
			rendered = m.styles.CommandStyle.Render(fmt.Sprintf("$ %s", msg.Content))
		case "output":
			rendered = m.styles.OutputStyle.Render(msg.Content)
		case "error":
			rendered = m.styles.ErrorStyle.Render(fmt.Sprintf("[%s] Error: %s", timestamp, msg.Content))
		}
		
		content.WriteString(rendered + "\n")
	}
	
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// Run starts the Bubble Tea interface
func (m *Model) Run() error {
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
