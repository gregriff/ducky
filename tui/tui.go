package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	"github.com/muesli/reflow/wordwrap"
)

type TUI struct {
	model models.LLM

	history []string
	vars    map[string]string

	SystemPrompt string
	ModelName    string
	TotalCost    float64
	MaxTokens    int
	responseChan chan string

	// UI state
	ready           bool
	viewport        viewport.Model
	input           string
	chatHistory     strings.Builder
	currentResponse strings.Builder
	isStreaming     bool
	renderer        *glamour.TermRenderer
	styles          styles
}

// Bubbletea messages
type streamChunk string
type streamComplete struct{}
type streamError struct{ err error }

func NewTUI(systemPrompt string, modelName string, maxTokens int) *TUI {
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithEmoji(),
		// glamour.WithWordWrap(80),
	)

	tui := &TUI{
		history: make([]string, 0),
		vars:    make(map[string]string),

		SystemPrompt: systemPrompt,
		ModelName:    modelName,
		TotalCost:    0.,
		MaxTokens:    maxTokens,
		responseChan: make(chan string),

		renderer: renderer,
		styles:   makeStyles(lipgloss.DefaultRenderer()),
	}

	// Add welcome message to chat history
	tui.addToChat("\nCommands: `:set <var> <value>`, `:get <var>`, `:history`, `:clear`, `:exit`\n\n---\n\n")

	return tui
}

func (t *TUI) Start() {
	p := tea.NewProgram(t,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

func (t *TUI) Init() tea.Cmd {
	return tea.SetWindowTitle("GPT-CLI")
}

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:

		msgString := msg.String()
		switch msgString {
		case "ctrl+d":
			return t, tea.Quit
		case "ctrl+u":
			t.input = ""
		case "esc":
			t.viewport.GotoBottom()
		}

		// TODO: impl cancel response WITH CONTEXTS
		// if t.isStreaming && msg.String() == "ctrl+c" {
		// 	t.isStreaming = false
		// 	t.addToChat(t.currentResponse.String() + "\n\n---\nResponse terminated\n---\n\n")
		// 	t.updateViewportContent()
		// 	t.currentResponse.Reset()
		// 	return r, nil
		// }

		if t.isStreaming {
			return t, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			return t.processUserInput()
		case tea.KeyBackspace:
			if len(t.input) > 0 {
				t.input = t.input[:len(t.input)-1]
			}
			return t, nil
		default:
			if IsValidPromptInput(msgString) {
				t.input += msgString
			}
			return t, nil
		}

	case streamChunk:
		t.currentResponse.WriteString(string(msg))
		t.updateViewportContent()
		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return t, waitForNextChunk(t.responseChan)

	case streamComplete:
		t.isStreaming = false
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		t.addToChat(t.currentResponse.String() + "\n\n---\n\n")
		t.currentResponse.Reset()
		t.updateViewportContent()
		return t, nil

	case streamError:
		t.isStreaming = false
		t.addToChat(t.currentResponse.String() + "\n\n---\n\n" + fmt.Sprintf("**Error:** %v\n\n---\n\n", msg.err))
		t.currentResponse.Reset()
		t.updateViewportContent()
		return t, nil

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(t.headerView())
		inputHeight := 3
		verticalMarginHeight := headerHeight + inputHeight

		if !t.ready {
			t.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			t.viewport.YPosition = headerHeight
			t.viewport.MouseWheelDelta = 1
			t.updateViewportContent()
			t.viewport.GotoBottom()
			t.ready = true
		} else {
			t.viewport.Width = msg.Width
			t.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle viewport updates
	t.viewport, cmd = t.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return t, tea.Batch(cmds...)
}

func (t *TUI) processUserInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(t.input)
	userInput := t.input
	t.input = ""

	if input == "" {
		return t, nil
	}

	// Add user input to chat
	t.addToChat(fmt.Sprintf("**You:** %s\n\n", userInput))

	if input == ":exit" || input == ":quit" {
		return t, tea.Quit
	}

	// Add to history
	t.history = append(t.history, input)

	// Process commands
	if result, isCommand := t.handleCommand(input); isCommand {
		if result != "" {
			t.addToChat(fmt.Sprintf("```\n%s\n```\n\n---\n\n", result))
		}
		t.updateViewportContent()
		t.viewport.GotoBottom()
		return t, nil
	}

	// Start LLM streaming
	if t.model == nil {
		t.model = anthropic.NewAnthropicModel(t.SystemPrompt, t.MaxTokens, t.ModelName, nil)
	}
	t.responseChan = make(chan string)
	t.isStreaming = true
	t.addToChat("**Assistant:** ")
	t.updateViewportContent()
	t.viewport.GotoBottom()

	return t, tea.Batch(
		// m.spinner.Tick,
		t.streamLLMResponse(input, t.responseChan),
		waitForNextChunk(t.responseChan),
	)
}

func (t *TUI) handleCommand(input string) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], ":") {
		return "", false
	}

	command := parts[0]
	switch command {
	case ":set":
		return t.handleSet(parts), true
	case ":get":
		return t.handleGet(parts), true
	case ":history":
		return t.showHistory(), true
	case ":clear":
		t.history = t.history[:0]
		t.chatHistory.Reset()
		t.addToChat("\nHistory cleared.\n\n---\n\n")
		return "", true
	case ":vars":
		return t.showVars(), true
	default:
		return "Unknown command", true
	}
}

func (t *TUI) streamLLMResponse(input string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		models.StreamPromptCompletion(t.model, input, true, ch)
		// Return nil to indicate this command is done - waitForNextChunk will handle the reading
		return nil
	}
}

func waitForNextChunk(ch chan string) tea.Cmd {
	return func() tea.Msg {
		if chunk, ok := <-ch; ok {
			return streamChunk(chunk)
		} else {
			return streamComplete{}
		}
	}
}

func (t *TUI) addToChat(content string) {
	t.chatHistory.WriteString(content)
}

func (t *TUI) updateViewportContent() {
	fullContent := t.chatHistory.String() + t.currentResponse.String()
	rendered, err := t.renderer.Render(fullContent)
	if err != nil {
		t.viewport.SetContent(wordwrap.String(fullContent, t.viewport.Width))
	} else {
		t.viewport.SetContent(wordwrap.String(rendered, t.viewport.Width))
	}
}

func (t *TUI) View() string {
	if !t.ready {
		return "\n  Initializing..."
	}

	return fmt.Sprintf("%s\n%s\n%s", t.headerView(), t.viewport.View(), t.inputView())
}

func (t *TUI) headerView() string {
	style := t.styles.TitleBar

	var title string
	if t.isStreaming {
		title = style.Render("GPT-CLI (streaming...)")
	} else {
		title = style.Render("GPT-CLI")
	}

	line := strings.Repeat("─", max(0, t.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (t *TUI) inputView() string {
	var prompt string
	if t.isStreaming {
		prompt = "Streaming response..."
	} else {
		prompt = fmt.Sprintf(" > %s", t.input)
	}

	return t.styles.InputArea.Render(prompt)
}
