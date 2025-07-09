package tui

import (
	"fmt"
	"log"
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

	// UI state
	ready    bool
	viewport viewport.Model

	// Chat state
	input       string
	isStreaming bool
	isReasoning bool

	currentResponse CurrentResponse
	history         ChatHistory
	responseChan    chan models.StreamChunk

	// Helpers
	md     *MarkdownRenderer
	styles Styles
}

type CurrentResponse struct {
	reasoningContent strings.Builder
	responseContent  strings.Builder
	errorContent     string
}

// isEmpty returns true if there is any text content or an error in the current response
func (res *CurrentResponse) isEmpty() bool {
	if res.Len() > 0 || len(res.errorContent) > 0 {
		return false
	}
	return true
}

// Len returns the total byte count of the resoning and response parts of the current response
func (res *CurrentResponse) Len() int {
	return res.reasoningContent.Len() + res.responseContent.Len()
}

// Bubbletea messages
type streamComplete struct{}

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, maxTokens int) *TUI {
	tui := &TUI{
		systemPrompt:    systemPrompt,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,

		history:      *NewChatHistory(),
		responseChan: make(chan models.StreamChunk),

		md:     NewMarkdownRenderer(),
		styles: makeStyles(),
	}

	tui.initLLMClient(modelName)
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
		text := string(msg.Content)

		if t.isReasoning {
			if !msg.Reasoning {
				t.isReasoning = false
				t.currentResponse.responseContent.WriteString(text)
			} else {
				t.currentResponse.reasoningContent.WriteString(text)
			}
		} else {
			t.currentResponse.responseContent.WriteString(text)
		}
		t.renderChat()
		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return t, t.waitForNextChunk()

	// TODO: include usage data by having DoStreamPromptCompletion return this with fields?
	case streamComplete: // responseChan guaranteed to be empty here
		t.isStreaming = false
		t.isReasoning = false
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		t.history.AddResponse(&t.currentResponse)
		t.renderChat()
		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		return t, nil

	case models.StreamError:
		log.Println("error event hit")
		t.currentResponse.errorContent = fmt.Sprintf("**Error:** %v\n\n---\n\n", msg.ErrMsg)
		return t, t.waitForNextChunk() // ensure last chunk is read and let chunk and complete messages handle state

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(t.headerView())
		inputHeight := 3
		verticalMarginHeight := headerHeight + inputHeight
		markdownWidth := msg.Width * 4 / 5

		if !t.ready {
			t.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			t.viewport.YPosition = headerHeight
			t.viewport.MouseWheelDelta = 2
			t.md.SetWidthImmediate(markdownWidth)
			t.renderChat()
			t.viewport.GotoBottom()
			t.ready = true
		} else {
			t.viewport.Width = msg.Width
			t.viewport.Height = msg.Height - verticalMarginHeight

			// TODO: here, the markdown renderer width is not updating before rendering happens. then the
			// viewport resize happens, still before the renderer changes width. consider forcing these to be in order
			// for smoother resizing
			t.md.SetWidth(markdownWidth)
			// t.md.SetWidthImmediate(msg.Width)
			t.history.ResizePrompts(msg.Width)
			t.renderChat()
		}
	}

	// Handle viewport updates
	t.viewport, cmd = t.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return t, tea.Batch(cmds...)
}

func (t *TUI) processUserInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(t.input)
	t.input = ""

	if input == "" {
		return t, nil
	}

	if input == ":exit" || input == ":quit" {
		return t, tea.Quit
	}

	// Process commands
	if _, isCommand := t.handleCommand(input); isCommand {
		t.renderChat() // needed for clear history
		t.viewport.GotoBottom()
		return t, nil
	}

	// Start LLM streaming
	return t.promptLLM(input)
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream
func (t *TUI) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	t.responseChan = make(chan models.StreamChunk)
	t.isStreaming = true
	if t.enableReasoning {
		t.isReasoning = true
	}

	t.history.AddPrompt(prompt, t.viewport.Width)
	t.renderChat()
	t.viewport.GotoBottom()

	return t, tea.Batch(
		// m.spinner.Tick,
		func() tea.Msg {
			return models.StreamPromptCompletion(t.model, prompt, t.enableReasoning, t.responseChan)
		},
		t.waitForNextChunk(),
	)
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

// renderChat renders the full chat history plus the current response in Markdown into the viewport
func (t *TUI) renderChat() {
	// TODO optimizations: impl an intersection system to only t.md.Render text that is within the viewport?
	fullChat := t.history.BuildChatString(t.md, &t.currentResponse)
	t.viewport.SetContent(fullChat)
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
