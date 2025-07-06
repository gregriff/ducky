package repl

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

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			Padding(0, 1)
)

type REPL struct {
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
}

// Bubbletea messages
type streamChunk string
type streamComplete struct{}
type streamError struct{ err error }

func NewREPL(systemPrompt string, modelName string, maxTokens int) *REPL {
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithEmoji(),
		// glamour.WithWordWrap(80),
	)

	repl := &REPL{
		history: make([]string, 0),
		vars:    make(map[string]string),

		SystemPrompt: systemPrompt,
		ModelName:    modelName,
		TotalCost:    0.,
		MaxTokens:    maxTokens,
		responseChan: make(chan string),

		renderer: renderer,
	}

	// Add welcome message to chat history
	repl.addToChat("\nCommands: `:set <var> <value>`, `:get <var>`, `:history`, `:clear`, `:exit`\n\n---\n\n")

	return repl
}

func (r *REPL) Start() {
	p := tea.NewProgram(r,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

func (r *REPL) Init() tea.Cmd {
	return tea.SetWindowTitle("GPT-CLI")
}

func (r *REPL) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:

		msgString := msg.String()
		switch msgString {
		case "ctrl+d":
			return r, tea.Quit
		case "ctrl+u":
			r.input = ""
		case "esc":
			r.viewport.GotoBottom()
		}

		// TODO: impl cancel response WITH CONTEXTS
		// if r.isStreaming && msg.String() == "ctrl+c" {
		// 	r.isStreaming = false
		// 	r.addToChat(r.currentResponse.String() + "\n\n---\nResponse terminated\n---\n\n")
		// 	r.updateViewportContent()
		// 	r.currentResponse.Reset()
		// 	return r, nil
		// }

		if r.isStreaming {
			return r, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			return r.processUserInput()
		case tea.KeyBackspace:
			if len(r.input) > 0 {
				r.input = r.input[:len(r.input)-1]
			}
			return r, nil
		default:
			if IsValidPromptInput(msgString) {
				r.input += msgString
			}
			return r, nil
		}

	case streamChunk:
		r.currentResponse.WriteString(string(msg))
		r.updateViewportContent()
		r.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return r, waitForNextChunk(r.responseChan)

	case streamComplete:
		r.isStreaming = false
		r.addToChat(r.currentResponse.String() + "\n\n---\n\n")
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		r.updateViewportContent()
		r.currentResponse.Reset()
		return r, nil

	case streamError:
		r.isStreaming = false
		r.addToChat(r.currentResponse.String() + "\n\n---\n\n" + fmt.Sprintf("**Error:** %v\n\n---\n\n", msg.err))
		r.updateViewportContent()
		r.currentResponse.Reset()
		return r, nil

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(r.headerView())
		inputHeight := 3
		verticalMarginHeight := headerHeight + inputHeight

		if !r.ready {
			r.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			r.viewport.YPosition = headerHeight
			r.viewport.MouseWheelDelta = 1
			r.updateViewportContent()
			r.viewport.GotoBottom()
			r.ready = true
		} else {
			r.viewport.Width = msg.Width
			r.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle viewport updates
	r.viewport, cmd = r.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return r, tea.Batch(cmds...)
}

func (r *REPL) processUserInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(r.input)
	userInput := r.input
	r.input = ""

	if input == "" {
		return r, nil
	}

	// Add user input to chat
	r.addToChat(fmt.Sprintf("**You:** %s\n\n", userInput))

	if input == ":exit" || input == ":quit" {
		return r, tea.Quit
	}

	// Add to history
	r.history = append(r.history, input)

	// Process commands
	if result, isCommand := r.handleCommand(input); isCommand {
		if result != "" {
			r.addToChat(fmt.Sprintf("```\n%s\n```\n\n---\n\n", result))
		}
		r.updateViewportContent()
		r.viewport.GotoBottom()
		return r, nil
	}

	// Start LLM streaming
	if r.model == nil {
		r.model = anthropic.NewAnthropicModel(r.SystemPrompt, r.MaxTokens, r.ModelName, nil)
	}
	r.responseChan = make(chan string)
	r.isStreaming = true
	r.addToChat("**Assistant:** ")
	r.updateViewportContent()
	r.viewport.GotoBottom()

	return r, tea.Batch(
		// m.spinner.Tick,
		r.streamLLMResponse(input, r.responseChan),
		waitForNextChunk(r.responseChan),
	)
}

func (r *REPL) handleCommand(input string) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], ":") {
		return "", false
	}

	command := parts[0]
	switch command {
	case ":set":
		return r.handleSet(parts), true
	case ":get":
		return r.handleGet(parts), true
	case ":history":
		return r.showHistory(), true
	case ":clear":
		r.history = r.history[:0]
		r.chatHistory.Reset()
		r.addToChat("\nHistory cleared.\n\n---\n\n")
		return "", true
	case ":vars":
		return r.showVars(), true
	default:
		return "Unknown command", true
	}
}

func (r *REPL) streamLLMResponse(input string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		models.StreamPromptCompletion(r.model, input, true, ch)
		return streamComplete{}
	}
}

func waitForNextChunk(ch chan string) tea.Cmd {
	return func() tea.Msg {
		if chunk, ok := <-ch; ok {
			return streamChunk(chunk)
		} else {
			return nil
		}
	}
}

func (r *REPL) addToChat(content string) {
	r.chatHistory.WriteString(content)
}

func (r *REPL) updateViewportContent() {
	fullContent := r.chatHistory.String() + r.currentResponse.String()
	rendered, err := r.renderer.Render(fullContent)
	if err != nil {
		r.viewport.SetContent(wordwrap.String(fullContent, r.viewport.Width))
	} else {
		r.viewport.SetContent(wordwrap.String(rendered, r.viewport.Width))
	}
}

func (r *REPL) View() string {
	if !r.ready {
		return "\n  Initializing..."
	}

	inputPrompt := r.inputView()
	return fmt.Sprintf("%s\n%s\n%s", r.headerView(), r.viewport.View(), inputPrompt)
}

func (r *REPL) headerView() string {
	var title string
	if r.isStreaming {
		title = titleStyle.Render("GPT-CLI (streaming...)")
	} else {
		title = titleStyle.Render("GPT-CLI")
	}

	line := strings.Repeat("─", max(0, r.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (r *REPL) inputView() string {
	var prompt string
	if r.isStreaming {
		prompt = "Streaming response..."
	} else {
		prompt = fmt.Sprintf(" > %s", r.input)
	}

	return inputStyle.Render(prompt)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
