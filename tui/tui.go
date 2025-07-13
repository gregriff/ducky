package tui

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	chat "github.com/gregriff/gpt-cli-go/tui/chat"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
	zone "github.com/lrstanley/bubblezone"
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

	// select+copy impl:
	// - only allow selecting text when not streaming
	// - pressing mouse down on viewport will enter select mode, mouse up will copy to clipboard (like tmux)
	// - ignore keyboard input while selecting
	// -
	selecting bool
	selectionLineStart,
	selectionLineEnd int

	// Chat state
	chat *chat.ChatModel
	isStreaming,
	isReasoning bool
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
			if t.textarea.Focused() {
				t.textarea.Blur()
			}
			return t.promptLLM(input)
		}

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			if t.isStreaming { // allow user to scroll up during streaming and keep their position
				t.preventScrollToBottom = true
			}
			break
		}

		switch msg.Action {
		case tea.MouseActionPress: // handles all mouse clicks
			if t.isStreaming || msg.Button != tea.MouseButtonLeft || t.selecting {
				return t, nil
			}

			textareaFocused := t.textarea.Focused()
			if zone.Get("chatViewport").InBounds(msg) {
				if textareaFocused && t.chat.HistoryLen() > 0 {
					t.textarea.Blur() // TODO: need to collapse it as well
				}
				// begin selecting text
				t.selecting = true
				t.selectionLineStart = t.mouseToPosition(msg.Y)
				log.Println("KEYDOWN: set selecting start:", t.selectionLineStart, "raw: ", msg.X, msg.Y)

			} else if zone.Get("promptInput").InBounds(msg) && !textareaFocused {
				t.textarea.Focus()
			}
		}

		// anything below this handles events while user is holding down the mouse to copy text
		if !t.selecting {
			break
		}

		// log.Println("entered selecting logic")

		switch msg.Action {
		case tea.MouseActionMotion: // handle dragging of mouse during selection
			// update copy state
			t.selectionLineEnd = t.mouseToPosition(msg.Y)
			log.Println("DRAG: set selection end: ", t.selectionLineEnd, "raw: ", msg.X, msg.Y)
		// TODO: render history with colored background around selection
		case tea.MouseActionRelease: // send to clipboard and reset state
			t.selectionLineEnd = t.mouseToPosition(msg.Y)
			log.Println("KEYUP: set selection end: ", t.selectionLineEnd)
			if text := t.getSelectedText(); len(text) > 0 {
				log.Println("\n\nSELECTED TEXT: ", text)
				clipboard.WriteAll(text)
			}
			t.selecting = false
			t.selectionLineStart = -1
			t.selectionLineEnd = -1
			// TODO: render normal history
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
		if !t.textarea.Focused() {
			t.textarea.Focus()
		}
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
		case "d", "u", "b", "j", "k":
			break
		}
	default:
		t.viewport, vpCmd = t.viewport.Update(msg)
	}
	return t, tea.Batch(tiCmd, vpCmd)
}

// Strip ANSI escape codes from text
func stripANSI(text string) string {
	// Regular expression to match ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiRegex.ReplaceAllString(text, "")
}

// mouseToPosition converts mouse coordinates to the line number within the main viewport
func (t *TUI) mouseToPosition(y int) int {
	vpContent := t.chat.Render(t.viewport.Width)
	vpContent = stripANSI(vpContent)
	lines := strings.Split(vpContent, "\n") // Split content into lines for position calculation
	line := max(y+t.viewport.YOffset, 0)

	if line >= len(lines) {
		line = len(lines) - 1
	}

	log.Println("mousetoPosition: lineNo:", line, "curLineLen:", len(lines[line]), "numLines:", len(lines))

	return line
}

// getSelectedText returns on-screen text selected by the user clicking and dragging their mouse
func (t *TUI) getSelectedText() string {
	start, end := t.selectionLineStart, t.selectionLineEnd

	vpContent := t.chat.Render(t.viewport.Width) // getSelectedText should never be called in a place where the width has changed
	vpContent = stripANSI(vpContent)
	lines := strings.Split(vpContent, "\n") // Split content into lines for selection extraction

	// Ensure start comes before end
	if start > end {
		start, end = end, start
	}

	// if lines[len(lines)-1] == ""

	// log.Println("GET TEXT: start,end:", start, end)
	// log.Println("GET TEXT: len(lines):", len(lines))
	// log.Println("GET TEXT: lines[start]:", lines[start])
	// log.Printf("\n%q", lines)

	var result []string
	for i := start; i <= end && i < len(lines); i++ {
		result = append(result, lines[i])
	}

	return strings.Join(result, "\n")
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
	return zone.Scan(
		fmt.Sprintf("%s\n%s\n%s",
			t.headerView(),
			zone.Mark("chatViewport", t.viewport.View()),
			zone.Mark("promptInput", t.textarea.View())),
	)
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
