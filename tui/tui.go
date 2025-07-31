package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/gregriff/ducky/models"
	"github.com/gregriff/ducky/models/anthropic"
	chat "github.com/gregriff/ducky/tui/chat"
	styles "github.com/gregriff/ducky/tui/styles"
	zone "github.com/lrstanley/bubblezone/v2"
)

type TUIModel struct {
	// user args TODO: combine these into a PromptContext struct (and add a context._), along with isStreaming + isReasoning?
	model           models.LLM
	systemPrompt    string
	maxTokens       int
	enableReasoning bool

	// UI state
	ready      bool
	textarea   textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	windowSize tea.WindowSizeMsg

	// Chat state
	chat *chat.ChatModel
	isStreaming,
	isReasoning bool
	responseChan chan models.StreamChunk

	preventScrollToBottom bool

	// rendering
	contentBuilder,
	headerBuilder strings.Builder
	lastWidth          int
	forceHeaderRefresh bool
}

// Bubbletea messages
type streamComplete struct{}

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, maxTokens int, glamourStyle string) *TUIModel {
	// create and style textarea
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // TODO: need this to be bound to shift+enter
	ta.Placeholder = "Send a prompt..."
	ta.Styles.Focused.Placeholder = styles.TUIStyles.PromptText
	ta.Styles.Focused.CursorLine = styles.TUIStyles.TextAreaCursor
	ta.Prompt = "┃ "
	ta.CharLimit = 100_000
	ta.SetHeight(styles.TEXTAREA_HEIGHT_NORMAL)

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = styles.TUIStyles.Spinner

	t := &TUIModel{
		systemPrompt:    systemPrompt,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,

		textarea: ta,
		spinner:  s,

		chat:         chat.NewChatModel(glamourStyle),
		responseChan: make(chan models.StreamChunk),
	}

	t.model = InitLLMClient(modelName, systemPrompt, maxTokens)
	return t
}

func (m *TUIModel) Start() {
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

// Init performs initial IO.
func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(tea.SetWindowTitle("ducky"), m.textarea.Focus())
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd,
		spCmd,
		vpCmd tea.Cmd

		cmds []tea.Cmd
	)

	// log.Printf("%#v", msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyString := msg.String()

		switch keyString {
		case "ctrl+d":
			return m, tea.Quit
		case "esc":
			// m.viewport.GotoBottom()
			// m.lastManualGoToBottom = time.Now()
			if m.textarea.Focused() {
				if m.textarea.Length() > 0 && m.chat.HistoryLen() > 0 {
					m.textarea.Blur()
					// TODO: if height is > normal, set height to normal
					cmds = append(cmds, m.redraw)
				}
			} else if !m.isStreaming {
				cmds = append(cmds, m.textarea.Focus(), m.redraw)
				// if numLines > curHeight:
				// 		if numLines > normal, set height to min(numLines, maxHeight)
				// 		else set height to normal
			}
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
		if m.isStreaming {
			break
		}

		// log.Println("STRING: ", keyString)

		switch keyString {
		case "ctrl+c":
			if m.chat.HistoryLen() == 0 {
				return m, tea.Quit
			}
			m.chat.Clear() // print something
			m.model.DoClearChatHistory()
			m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
			if !m.textarea.Focused() {
				return m, m.textarea.Focus()
			}
			return m, nil
		case "enter":
			input := strings.TrimSpace(m.textarea.Value())
			m.textarea.Reset()

			if input == "" {
				return m, nil
			}

			// Start LLM streaming
			return m.promptLLM(input)
		}

	case tea.PasteMsg:
		if m.isStreaming { // don't allow paste while streaming
			return m, nil
		}
		// here we grab the paste message before textarea gets it, in order to increase the height of the textarea if
		// the pasted text has many lines
		content, _ := clipboard.ReadAll()
		newlines := strings.Count(content, "\n")
		if newlines > m.textarea.Height() {
			newHeight := clamp(newlines, styles.TEXTAREA_HEIGHT_NORMAL, m.textarea.MaxHeight)
			windowHeight, windowWidth := m.windowSize.Height, m.windowSize.Width
			viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth, &newHeight)

			m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
			m.textarea.SetHeight(newHeight)      // this func clamps
			return m.updateComponents(msg, cmds) // pass the paste msg to the textarea
		}
	case tea.MouseMsg:
		var (
			scrollCmd     tea.Cmd
			scrollKey     tea.KeyMsg
			triggerScroll bool
		)
		mouse := msg.Mouse()

		switch msg := msg.(type) {
		case tea.MouseClickMsg:
			// TODO: add right-click functionality
			if m.isStreaming || msg.Button != tea.MouseLeft {
				return m, nil
			}

			textareaFocused := m.textarea.Focused()
			if zone.Get("chatViewport").InBounds(msg) {
				if m.chat.HistoryLen() == 0 {
					break // could just return m, nil
				}
				// this allows the user to click the viewport and not have the textarea be unfocused if theres not a lot of text in it
				if textareaFocused && m.textarea.LineCount() > styles.TEXTAREA_HEIGHT_COLLAPSED {
					m.textarea.Blur() // TODO: need to collapse it as well
				}

			} else if zone.Get("promptInput").InBounds(msg) {
				if !textareaFocused {
					return m, m.textarea.Focus()
				}
			}
		case tea.MouseWheelMsg:
			switch mouse.Button {
			case tea.MouseWheelUp:
				// here we don't scroll up if the user has just pressed esc. On mac, the rapid scroll events build up, and may
				// register after the esc handler, which results in the viewport scrolling up after going to the bottom.
				// if time.Since(m.lastManualGoToBottom) < 800*time.Millisecond {
				// return m, nil
				// }
				if m.isStreaming { // allow user to scroll up during streaming and keep their position
					m.preventScrollToBottom = true
				}
				triggerScroll, scrollKey = true, tea.KeyPressMsg{Code: tea.KeyUp}

			case tea.MouseWheelDown:
				triggerScroll, scrollKey = true, tea.KeyPressMsg{Code: tea.KeyDown}
			}

			if triggerScroll {
				if m.textarea.Focused() {
					if m.textarea.LineCount() <= m.textarea.Height() {
						m.viewport, scrollCmd = m.viewport.Update(msg)
					} else {
						m.textarea, scrollCmd = m.textarea.Update(scrollKey)
					}
				} else {
					m.viewport, scrollCmd = m.viewport.Update(msg)
				}
				return m, scrollCmd
			}
		}

	case tea.BlurMsg:
		if m.textarea.Focused() {
			m.textarea.Blur()
		}
		return m, nil

	// NOTE: on tmux, regaining focus from switching panes results in `tea.unknownCSISequenceMsg{0x1b, 0x5b, 0x49}`, so this is not run
	case tea.FocusMsg:
		return m, m.textarea.Focus()

	case models.StreamChunk:
		m.isReasoning = msg.Reasoning
		m.chat.AccumulateStream(msg.Content, msg.Reasoning, false)

		m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
		if !m.preventScrollToBottom {
			m.viewport.GotoBottom()
		}
		return m, m.waitForNextChunk

	// TODO: include usage data by having DoStreamPromptCompletion return this with fields?
	case streamComplete: // responseChan guaranteed to be empty here
		// if a StreamError occurs before response streaming begins, two waitForNextChunks will return streamComplete
		if m.isStreaming == false {
			return m, nil
		}
		m.isStreaming = false
		m.isReasoning = false
		m.forceHeaderRefresh = true
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		m.chat.AddResponse()

		m.viewport.SetContent(m.chat.Render(m.viewport.Width()))

		if !m.preventScrollToBottom {
			m.viewport.GotoBottom()
		}
		m.preventScrollToBottom = false
		if !m.textarea.Focused() {
			return m, m.textarea.Focus()
		}
		return m, nil

	case models.StreamError:
		errMsg := fmt.Sprintf("**Error:** %v", msg.ErrMsg)
		m.chat.AccumulateStream(errMsg, false, true)
		return m, m.waitForNextChunk // ensure last chunk is read and let chunk and complete messages handle state

	case spinner.TickMsg:
		if m.isStreaming {
			m.spinner, spCmd = m.spinner.Update(msg)
		}
		return m, spCmd

	case tea.WindowSizeMsg:
		m.windowSize = msg
		windowHeight, windowWidth := msg.Height, msg.Width
		viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth, nil)

		// TODO: should be able to move this into constructor, and style Viewport with vp.Style
		if !m.ready {
			m.viewport = viewport.New(viewport.WithWidth(windowWidth), (viewport.WithHeight(viewportHeight)))
			m.viewport.MouseWheelDelta = 2
			m.viewport.SetContent(m.chat.Render(windowWidth))
			m.viewport.GotoBottom()
			m.textarea.SetWidth(textAreaWidth)
			m.textarea.MaxHeight = viewportHeight / 2
			m.ready = true
		} else {
			m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
		}
		return m.updateComponents(msg, cmds)
	}

	// TODO: resizing window while textarea is not focused may prevent textarea resizing until it is focused
	if m.textarea.Focused() {
		var newHeight int
		expanded, collapsed := styles.TEXTAREA_HEIGHT_NORMAL, styles.TEXTAREA_HEIGHT_COLLAPSED
		if m.textarea.Length() > 0 {
			if m.textarea.Height() < expanded {
				newHeight = expanded
			}
		} else if m.textarea.Height() > collapsed {
			newHeight = collapsed
		}

		// set height of textarea, updating viewport first to prevent visual glitching
		if newHeight != 0 {
			windowHeight, windowWidth := m.windowSize.Height, m.windowSize.Width
			viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth, &newHeight)

			m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
			m.textarea.SetHeight(newHeight)
			return m.updateComponents(msg, cmds)
		}

		// This runs when the textarea is focused and not being resized.
		// NOTE: this prevents messages from reaching the viewport, which may not be desirable
		// ensure we aren't returning nil above these lines and therefore blocking messages to these models
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
		return m, tea.Batch(cmds...)
	}

	// prevent movement keys from scrolling the viewport
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "d", "u", "b", "j", "k":
			break
		}
	default:
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}
	return m, tea.Batch(cmds...)
}

// updateComponents sends a Msg and []Cmd to the viewport and textarea to update their state and returns a Batch of all commands.
// Use this in the Update function when both components need to be updated
func (m *TUIModel) updateComponents(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	var taCmd, vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd, vpCmd)
	return m, tea.Batch(cmds...)
}

// redraw initiates the Window resize handler. Use it after changing the dimensions of a component to make the others update
func (m *TUIModel) redraw() tea.Msg {
	return m.windowSize
}

func (m *TUIModel) resizeComponents(windowWidth, textAreaWidth, viewportHeight int) {
	m.viewport.SetWidth(windowWidth)
	m.viewport.SetHeight(viewportHeight)

	m.textarea.SetWidth(textAreaWidth)
	m.viewport.SetContent(m.chat.Render(windowWidth))
}

// getResizeParams returns size dimensions of on-screen components needed during redrawing or resizing
func (m *TUIModel) getResizeParams(windowHeight, windowWidth int, taHeight *int) (viewportHeight int, textAreaWidth int) {
	var textAreaHeight int
	if taHeight != nil {
		textAreaHeight = *taHeight
	} else {
		textAreaHeight = m.textarea.Height()
	}

	headerHeight := lipgloss.Height(m.headerView(m.viewport.Width()))
	verticalMarginHeight := headerHeight + textAreaHeight + styles.VP_TA_SPACING_SIZE

	viewportHeight = windowHeight - verticalMarginHeight
	textAreaWidth = windowWidth - styles.H_PADDING
	return viewportHeight, textAreaWidth
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream
func (m *TUIModel) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	m.responseChan = make(chan models.StreamChunk)
	m.isStreaming = true
	if m.enableReasoning && m.model.DoesSupportReasoning() {
		m.isReasoning = true
	}

	if m.textarea.Focused() {
		m.textarea.Blur()
	}

	m.chat.AddPrompt(prompt)
	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
	m.viewport.GotoBottom()
	m.textarea.SetHeight(styles.TEXTAREA_HEIGHT_COLLAPSED)

	beginStreaming := func() tea.Msg {
		return models.StreamPromptCompletion(m.model, prompt, m.enableReasoning, m.responseChan)
	}

	return m, tea.Batch(
		m.spinner.Tick,
		m.redraw, // recalculate view because we've changed the textarea height
		beginStreaming,
		m.waitForNextChunk,
	)
}

// waitForNextChunk notifies the Update function when a response chunk arrives, and also when the response is completed.
func (m *TUIModel) waitForNextChunk() tea.Msg {
	if chunk, ok := <-m.responseChan; ok {
		return chunk
	} else {
		return streamComplete{}
	}

}

func (m *TUIModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	m.contentBuilder.Reset()
	m.contentBuilder.WriteString(
		zone.Scan(
			fmt.Sprintf("%s\n%s\n%s",
				m.headerView(m.viewport.Width()),
				zone.Mark("chatViewport", m.viewport.View()),
				zone.Mark("promptInput", styles.VP_TA_SPACING+m.textarea.View()),
			),
		))
	return m.contentBuilder.String()
}

// headerView returns the formatted header, reusing the last computed headerView result if the width hasn't changed and the spinner doesn't
// need to be updated
func (m *TUIModel) headerView(width int) string {
	var leftText string
	if !m.isStreaming {
		if width == m.lastWidth && !m.forceHeaderRefresh {
			return m.headerBuilder.String()
		}
		leftText = "ducky"
	} else {
		leftText = m.spinner.View()
	}
	m.headerBuilder.Reset()
	m.lastWidth = width

	if m.forceHeaderRefresh {
		m.forceHeaderRefresh = false
	}

	rightText := models.GetModelId(m.model)
	titleTextWidth := lipgloss.Width(leftText) +
		lipgloss.Width(rightText) +
		styles.H_PADDING*2 + // the left and right padding defined in TUIStyles.TitleBar
		2 // the two border chars

	// TODO: should we be using termWidth or viewportWidth?
	width = max(0, width)
	style := styles.TUIStyles.TitleBar.Width(width)
	spacing := strings.Repeat(" ", max(5, width-titleTextWidth))

	m.headerBuilder.WriteString(
		style.Render(lipgloss.JoinHorizontal(lipgloss.Center, leftText, spacing, rightText)),
	)
	return m.headerBuilder.String()

}

// InitLLMClient creates an LLM Client given a modelName. It is called at TUI init, and can be called any time later
// in order to switch between LLMs while preserving message history
func InitLLMClient(modelName, systemPrompt string, maxTokens int) models.LLM {
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
				return anthropic.NewModel(sysPrompt, maxTokens, name, msgs)
			},
		},
		// {openai.ValidateModelName, openai.NewOpenAIModel},
	}

	for _, provider := range providers {
		if err := provider.validateFunc(modelName); err == nil {
			// does not return an error, should it? Also, any cleanup we need to do?
			return provider.newModelFunc(systemPrompt, maxTokens, modelName, nil)
		}
	}
	return nil
	// return fmt.Errorf("unsupported model: %s", modelName)
}

// clamp is a copy/pasted func from bubbles/textarea, in order to replicate its internal behavior
func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
