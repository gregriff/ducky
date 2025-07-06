package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	"github.com/muesli/reflow/wordwrap"
)

// TODO: group some fields into sub-structs (rendererManager)
type TUI struct {
	model        models.LLM
	ModelId      string
	SystemPrompt string
	ModelName    string
	TotalCost    float64
	MaxTokens    int

	history []string
	vars    map[string]string

	// UI state
	styles          Styles
	ready           bool
	viewport        viewport.Model
	input           string
	chatHistory     strings.Builder
	currentResponse strings.Builder
	responseChan    chan string
	isStreaming     bool
	renderMgr       *RendererManager
}

// Bubbletea messages
type streamChunk string
type streamComplete struct{}
type streamError struct{ err error }

func NewTUI(systemPrompt string, modelName string, maxTokens int) *TUI {
	tui := &TUI{
		SystemPrompt: systemPrompt,
		ModelName:    modelName,
		TotalCost:    0.,
		MaxTokens:    maxTokens,

		history: make([]string, 0),
		vars:    make(map[string]string),

		styles:       makeStyles(lipgloss.DefaultRenderer()),
		responseChan: make(chan string),
		renderMgr:    NewRendererManager(),
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
			t.renderMgr.ForceCreation(msg.Width)
			t.updateViewportContent()
			t.viewport.GotoBottom()
			t.ready = true
		} else {
			t.viewport.Width = msg.Width
			t.viewport.Height = msg.Height - verticalMarginHeight
			t.renderMgr.SetWidth(msg.Width)
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
	// the below runs in a goroutine. It will immediately return, and our Update func has already queued a waitForNextChunk to
	// receive on the responseChannel
	return func() tea.Msg {
		models.StreamPromptCompletion(t.model, input, true, ch)
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
	rendered, err := t.renderMgr.Render(fullContent)
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
			t.model = provider.newModelFunc(t.SystemPrompt, t.MaxTokens, modelName, nil)
			t.ModelId = models.GetModelId(t.model)
			t.ModelName = modelName
			return nil
		}
	}

	return fmt.Errorf("unsupported model: %s", modelName)
}
