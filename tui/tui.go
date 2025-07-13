package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	chat "github.com/gregriff/gpt-cli-go/tui/chat"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
)

type TUI struct {
	styles *styles.TUIStylesStruct

	// user args TODO: combine these into a PromptContext struct (and add a context._), along with isStreaming + isReasoning?
	model           models.LLM
	systemPrompt    string
	maxTokens       int
	enableReasoning bool

	// UI state
	ready    bool
	textarea textarea.Model
	viewport viewport.Model

	// Chat state
	chat         *chat.ChatModel
	isStreaming  bool
	isReasoning  bool
	responseChan chan models.StreamChunk

	preventScrollToBottom bool
}

// Bubbletea messages
type streamComplete struct{}

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, maxTokens int, glamourStyle string) *TUI {
	// create and style textarea
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // TODO: need this to be bound to shift+enter
	ta.Placeholder = "Send a prompt..."
	ta.FocusedStyle.Placeholder = styles.TUIStyles.PromptText
	ta.FocusedStyle.CursorLine = styles.TUIStyles.TextAreaCursor
	ta.Prompt = "┃ "
	ta.CharLimit = -1
	ta.Focus()
	ta.SetHeight(4)

	t := &TUI{
		styles: &styles.TUIStyles,

		systemPrompt:    systemPrompt,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,

		textarea: ta,

		chat:         chat.NewChatModel(glamourStyle),
		responseChan: make(chan models.StreamChunk),
	}

	t.initLLMClient(modelName)
	return t
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
	return tea.Batch(tea.SetWindowTitle("GPT-CLI"), textarea.Blink)
}

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd,
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyString := msg.String()

		switch keyString {
		case "ctrl+d":
			return t, tea.Quit
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
			break
		}

		switch keyString {
		case "ctrl+c":
			if t.chat.HistoryLen() == 0 {
				return t, tea.Quit
			}
			t.chat.Clear() // print something
			t.viewport.SetContent(t.chat.Render(t.viewport.Width))
			return t, nil
		case "enter":
			input := strings.TrimSpace(t.textarea.Value())
			t.textarea.Reset()

			if input == "" {
				return t, nil
			}

			// Start LLM streaming
			t.textarea.Blur()
			return t.promptLLM(input)
		}

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			if t.isStreaming { // allow user to scroll up during streaming and keep their position
				t.preventScrollToBottom = true
			}
		}

	case models.StreamChunk:
		if t.isReasoning {
			if !msg.Reasoning {
				t.isReasoning = false
				t.chat.CurrentResponse.ResponseContent.WriteString(msg.Content)
			} else {
				t.chat.CurrentResponse.ReasoningContent.WriteString(msg.Content)
			}
		} else {
			t.chat.CurrentResponse.ResponseContent.WriteString(msg.Content)
		}
		t.viewport.SetContent(t.chat.Render(t.viewport.Width))
		if !t.preventScrollToBottom {
			t.viewport.GotoBottom()
		}
		return t, t.waitForNextChunk()

	// TODO: include usage data by having DoStreamPromptCompletion return this with fields?
	case streamComplete: // responseChan guaranteed to be empty here
		// if a StreamError occurs before response streaming begins, two waitForNextChunks will return streamComplete
		if t.isStreaming == false {
			return t, nil
		}
		t.isStreaming = false
		t.isReasoning = false
		t.preventScrollToBottom = false
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		t.chat.AddResponse()

		t.viewport.SetContent(t.chat.Render(t.viewport.Width))

		t.viewport.GotoBottom() // TODO: dont run this if user has scrolled up during response streaming (wants to read)
		t.textarea.Focus()
		return t, nil

	case models.StreamError:
		t.chat.CurrentResponse.ErrorContent = fmt.Sprintf("**Error:** %v", msg.ErrMsg)
		return t, t.waitForNextChunk() // ensure last chunk is read and let chunk and complete messages handle state

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(t.headerView())
		verticalMarginHeight := headerHeight + t.textarea.Height()
		markdownWidth := int(float64(msg.Width) * styles.RESPONSE_WIDTH_PROPORTION)

		if !t.ready {
			t.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			t.viewport.YPosition = headerHeight
			t.viewport.MouseWheelDelta = 2
			t.chat.Markdown.SetWidthImmediate(markdownWidth)
			t.viewport.SetContent(t.chat.Render(msg.Width))
			t.viewport.GotoBottom()
			t.textarea.SetWidth(msg.Width - styles.H_PADDING)
			t.ready = true
		} else {
			t.viewport.Width = msg.Width
			t.viewport.Height = msg.Height - verticalMarginHeight

			// TODO: here, the markdown renderer width is not updating before rendering happens. then the
			// viewport resize happens, still before the renderer changes width. consider forcing these to be in order
			// for smoother resizing
			t.chat.Markdown.SetWidth(markdownWidth)
			// t.md.SetWidthImmediate(msg.Width)
			t.textarea.SetWidth(msg.Width - styles.H_PADDING)
			t.viewport.SetContent(t.chat.Render(msg.Width))
		}
	}

	// ensure we aren't returning nil above these lines and therefore blocking messages to these models
	t.textarea, tiCmd = t.textarea.Update(msg)

	// prevent movement keys from scrolling the viewport
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			break
		case "u":
			break
		case "b":
			break
		case "j":
			break
		case "k":
			break
		}
	default:
		t.viewport, vpCmd = t.viewport.Update(msg)
	}
	return t, tea.Batch(tiCmd, vpCmd)
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream
func (t *TUI) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	t.responseChan = make(chan models.StreamChunk)
	t.isStreaming = true
	if t.enableReasoning { // TODO: && model.supportsReasoning (make new interface func)
		t.isReasoning = true
	}

	t.chat.AddPrompt(prompt)
	t.viewport.SetContent(t.chat.Render(t.viewport.Width))
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

func (t *TUI) View() string {
	if !t.ready {
		return "Initializing..."
	}

	return fmt.Sprintf("%s\n%s\n%s", t.headerView(), t.viewport.View(), t.textarea.View())
}

func (t *TUI) headerView() string {
	leftText := "GPT-CLI"
	rightText := models.GetModelId(t.model)
	if t.isStreaming {
		leftText += " (streaming...)" // TODO: loading spinner
	}
	maxWidth := t.viewport.Width - styles.HEADER_R_PADDING
	titleTextWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText) + 2 // the two border chars
	spacing := strings.Repeat(" ", max(5, maxWidth-titleTextWidth))

	return t.styles.TitleBar.Width(max(0, maxWidth)).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, leftText, spacing, rightText))

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
