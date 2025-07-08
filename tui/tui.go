package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
)

// TODO: group some fields into sub-structs (rendererManager)
type TUI struct {
	model           models.LLM
	systemPrompt    string
	totalCost       float64
	maxTokens       int
	enableReasoning bool

	history []string
	vars    map[string]string

	// UI state
	ready    bool
	viewport viewport.Model

	// Chat state
	input           string
	isStreaming     bool
	isReasoning     bool
	chatHistory     strings.Builder
	currentResponse strings.Builder
	responseChan    chan models.StreamChunk

	// Helpers
	md     *MarkdownRenderer
	styles Styles
}

// Bubbletea messages
type streamComplete struct{}
type streamError struct{ err error }

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, maxTokens int) *TUI {
	tui := &TUI{
		systemPrompt: systemPrompt,
		// totalCost:      0.,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,

		history: make([]string, 0),
		vars:    make(map[string]string),

		styles:       makeStyles(),
		md:           NewMarkdownRenderer(),
		responseChan: make(chan models.StreamChunk),
	}

	tui.initLLMClient(modelName)

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

// Init performs initial IO.
func (t *TUI) Init() tea.Cmd {
	initMarkdownRenderer := func() tea.Msg {
		t.md.SetWidthImmediate(0)
		return nil
	}
	return tea.Batch(initMarkdownRenderer, tea.SetWindowTitle("GPT-CLI"))
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

		// while streaming, anything below this will not be accessible
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

	case models.StreamChunk:
		if t.isReasoning && !msg.Reasoning {
			t.isReasoning = false

			// this will catch if the model did not support reasoning
			if t.currentResponse.Len() > 0 {
				// NOTE: the number of newlines surrounding a block of text will style it differently.
				t.currentResponse.WriteString("\n\n---\n")
			}
		}
		t.currentResponse.WriteString(string(msg.Content))
		t.updateViewportContent()
		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return t, t.waitForNextChunk()

	// TODO: include usage data?
	case streamComplete:
		t.isStreaming = false
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		t.addToChat(t.currentResponse.String() + "\n\n---\n\n")
		t.currentResponse.Reset()
		t.updateViewportContent()
		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return t, nil

	case streamError:
		t.isStreaming = false
		t.isReasoning = false
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
			t.md.SetWidthImmediate(msg.Width)
			t.updateViewportContent()
			t.viewport.GotoBottom()
			t.ready = true
		} else {
			t.viewport.Width = msg.Width
			t.viewport.Height = msg.Height - verticalMarginHeight
			t.md.SetWidth(msg.Width)
			t.updateViewportContent()
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
	t.responseChan = make(chan models.StreamChunk)
	prompt := input
	t.isStreaming = true
	if t.enableReasoning {
		t.isReasoning = true
	}

	t.addToChat("**Assistant:** ")
	t.updateViewportContent()
	t.viewport.GotoBottom()

	return t, tea.Batch(
		// m.spinner.Tick,
		t.streamLLMResponse(prompt, t.enableReasoning),
		t.waitForNextChunk(),
	)
}

// streamLLMResponse sends an API request to get a response from an LLM and sends chunked updates to the two channels
func (t *TUI) streamLLMResponse(prompt string, enableReasoning bool) tea.Cmd {
	// the below runs in a goroutine. It will immediately return, and our Update func has already queued a waitForNextChunk to
	// receive on the responseChannel
	return func() tea.Msg {
		models.StreamPromptCompletion(t.model, prompt, enableReasoning, t.responseChan)
		return nil // should this return streamComplete?
	}
}

// waitForNextChunk notifies the Update function when a response chunk arrives, and also when the response is completed.
func (t *TUI) waitForNextChunk() tea.Cmd {
	return func() tea.Msg {
		if chunk, ok := <-t.responseChan; ok {
			return chunk
		} else {
			return streamComplete{}
		}
	}
}

func (t *TUI) addToChat(content string) {
	t.chatHistory.WriteString(content)
}

// updateViewportContent renders the full chat history plus the current response in Markdown into the viewport
func (t *TUI) updateViewportContent() {
	fullContent := t.chatHistory.String() + t.currentResponse.String()
	t.viewport.SetContent(t.md.Render(fullContent))
}

func (t *TUI) View() string {
	if !t.ready {
		return "Initializing..."
	}

	return fmt.Sprintf("%s\n%s\n%s", t.headerView(), t.viewport.View(), t.inputView())
}

func (t *TUI) headerView() string {
	// TODO: my alacritty term is cropping the term window so i need this
	const R_PADDING int = H_PADDING * 2

	leftText := "GPT-CLI"
	rightText := models.GetModelId(t.model)
	if t.isStreaming {
		leftText += " (streaming...)" // TODO: loading spinner
	}
	maxWidth := t.viewport.Width - R_PADDING
	titleTextWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText) + 2 // the two border chars
	spacing := strings.Repeat(" ", max(5, maxWidth-titleTextWidth))

	style := t.styles.TitleBar.Width(max(0, maxWidth))
	return style.Render(lipgloss.JoinHorizontal(lipgloss.Center, leftText, spacing, rightText))
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

// initLLMClient creates an LLM Client given a modelName. It is called at TUI init, and can be called any time later
// in order to switch between LLMs while preserving message history
func (t *TUI) initLLMClient(modelName string) error {
	// var pastMessages []models.Message
	// if t.model != nil {
	// pastMessages = t.model.DoGetChatHistory()
	// }

	// Try each provider in order
	// TODO: improve with the LLM interface.
	// NOTE: newModelFunc will have different signature for openAI (topP etc). will need to use optional params maybe
	providers := []struct {
		validateFunc func(string) error
		newModelFunc func(string, int, string, *[]models.Message) models.LLM
	}{
		{
			anthropic.ValidateModelName,
			func(sysPrompt string, maxTokens int, name string, msgs *[]models.Message) models.LLM {
				return anthropic.NewAnthropicModel(sysPrompt, maxTokens, name, msgs)
			},
		},
		// {openai.ValidateModelName, openai.NewOpenAIModel},
	}

	for _, provider := range providers {
		if err := provider.validateFunc(modelName); err == nil {
			// does not return an error, should it? Also, any cleanup we need to do?
			t.model = provider.newModelFunc(t.systemPrompt, t.maxTokens, modelName, nil)
			return nil
		}
	}

	return fmt.Errorf("unsupported model: %s", modelName)
}
