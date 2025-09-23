package internal

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	chat "github.com/gregriff/ducky/internal/chat"
	"github.com/gregriff/ducky/internal/math"
	"github.com/gregriff/ducky/internal/models"
	"github.com/gregriff/ducky/internal/models/anthropic"
	"github.com/gregriff/ducky/internal/models/openai"
	styles "github.com/gregriff/ducky/internal/styles"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/muesli/reflow/wordwrap"
)

type TUIModel struct {
	// user args TODO: combine these into a PromptContext struct (and add a context._), along with isStreaming + isReasoning?
	model           models.LLM
	systemPrompt    string
	maxTokens       int
	enableReasoning bool
	reasoningEffort *uint8
	initialPrompt   string // if stdin is a pipe and --force-interactive is used

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
type (
	makeInitialPrompt struct{}
	streamComplete    struct{}
)

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, reasoningEffort *uint8, maxTokens int, glamourStyle string) *TUIModel {
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

func (m *TUIModel) Start(initialPrompt string) {
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)
	m.initialPrompt = initialPrompt
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

// Init performs initial IO.
func (m *TUIModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle("ducky"),
		m.textarea.Focus(),
	}

	if len(m.initialPrompt) > 0 {
		cmds = append(cmds, func() tea.Msg {
			return makeInitialPrompt{}
		})
	}
	return tea.Batch(cmds...)
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var spCmd,
		vpCmd tea.Cmd

	// log.Printf("\n\nMESSAGE RECEIVED: %#v", msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyString := msg.String()

		switch keyString {
		case "ctrl+d":
			return m, tea.Quit
		case "esc":
			return m.handleEscape()
		case "up", "down":
			// if not allowed, arrow key inputs will be handled by the textarea if its focused
			if allow := m.allowScrollback(keyString); !allow {
				break
			}
			return m.triggerScrollback(msg)
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

		switch keyString {
		case "ctrl+c":
			return m.handleCtrlC()
		case "enter":
			return m.handleEnter()
		}

	case tea.PasteMsg:
		if m.isStreaming { // don't allow paste while streaming
			return m, nil
		}
		// here we grab the paste message before textarea gets it, in order to increase the height of the textarea if
		// the pasted text has many lines
		content, _ := clipboard.ReadAll()
		wrappedLineCount := m.getNumLines(content)
		if wrappedLineCount > m.textarea.Height() {
			newHeight := math.Clamp(wrappedLineCount, styles.TEXTAREA_HEIGHT_NORMAL, m.textarea.MaxHeight)
			windowHeight, windowWidth := m.windowSize.Height, m.windowSize.Width
			viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth, &newHeight)

			m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
			m.textarea.SetHeight(newHeight) // this func clamps
			return m.updateComponents(msg)  // pass the paste msg to the textarea
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
				if textareaFocused && m.getNumLines(m.textarea.Value()) > styles.TEXTAREA_HEIGHT_COLLAPSED {
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

			// if the mousewheel button is not scroll up or scroll down
			if !triggerScroll {
				break
			}

			if m.textarea.Focused() {
				wrappedLineCount := m.getNumLines(m.textarea.Value())
				taHeight := m.textarea.Height()
				if wrappedLineCount < taHeight || taHeight == styles.TEXTAREA_HEIGHT_COLLAPSED {
					m.viewport, scrollCmd = m.viewport.Update(msg)
				} else {
					m.textarea, scrollCmd = m.textarea.Update(scrollKey)
				}
			} else {
				m.viewport, scrollCmd = m.viewport.Update(msg)
			}
			return m, scrollCmd
		}

	case tea.BlurMsg:
		if m.textarea.Focused() {
			m.textarea.Blur()
		}
		return m, nil

	// NOTE: on tmux, regaining focus from switching panes results in `tea.unknownCSISequenceMsg{0x1b, 0x5b, 0x49}`, so this is not run
	case tea.FocusMsg:
		return m, m.textarea.Focus()

	case makeInitialPrompt:
		return m.promptLLM(m.initialPrompt)

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
		return m.handleStreamComplete()

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
		return m.handleWindowResize(msg)
	}

	// TODO: resizing window while textarea is not focused may prevent textarea resizing until it is focused
	if m.textarea.Focused() {
		// in this current state, the textarea intercepts all commands from the viewport if its focused.
		// this may not be desirable. TODO: if focus switches mid-scroll, ignore scroll commands?
		return m.updateTextarea(msg)
	}

	// prevent movement keys from scrolling the viewport
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Key().Text {
		case "d", "u", "b", "j", "k":
			break
		}
	default:
		m.viewport, vpCmd = m.viewport.Update(msg)
		return m, vpCmd
	}
	return m, nil
}

// updateComponents sends a Msg and []Cmd to the viewport and textarea to update their state and returns a Batch of all commands.
// Use this in the Update function when both components need to be updated
func (m *TUIModel) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: can we just move this into the resizeComponents func?
	var taCmd, vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, tea.Batch(taCmd, vpCmd)
}

// redraw initiates the Window resize handler. Use it after changing the dimensions of a component to make the others update
func (m *TUIModel) redraw() tea.Msg {
	return m.windowSize
}

// resizeComponents sets properties on the viewport and textarea to resize them on their next Update()
func (m *TUIModel) resizeComponents(windowWidth, textAreaWidth, viewportHeight int) {
	m.viewport.SetWidth(windowWidth)
	m.viewport.SetHeight(viewportHeight)

	m.textarea.MaxWidth = textAreaWidth
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

func (m *TUIModel) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.windowSize = msg
	windowHeight, windowWidth := msg.Height, msg.Width
	viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth, nil)

	// TODO: should be able to move this into constructor, and style Viewport with vp.Style
	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(windowWidth), (viewport.WithHeight(viewportHeight)))
		m.viewport.MouseWheelDelta = 2 // TODO: make this configurable
		m.viewport.SetContent(m.chat.Render(windowWidth))
		m.viewport.GotoBottom()
		m.textarea.MaxWidth = textAreaWidth
		m.textarea.SetWidth(textAreaWidth)
		m.textarea.MaxHeight = viewportHeight / 2
		m.ready = true
	} else {
		m.textarea.MaxHeight = viewportHeight / 2
		m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
	}
	return m.updateComponents(msg)
}

// getNumLines returns the number of lines the text in the textarea takes up (soft-wrapped).
// takes into account lines of text not visible on screen (scrolled out of view)
func (m *TUIModel) getNumLines(text string) int {
	wrapped := wordwrap.String(text, m.textarea.MaxWidth)
	lines := strings.Split(wrapped, "\n")
	return len(lines)
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
		return models.StreamPromptCompletion(m.model, prompt, m.enableReasoning, m.reasoningEffort, m.responseChan)
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
	}
	return streamComplete{}
}

// handleStreamComplete updates TUI state when a LLM response has been fully received
func (m *TUIModel) handleStreamComplete() (tea.Model, tea.Cmd) {
	// if a StreamError occurs before response streaming begins, two waitForNextChunks will return streamComplete
	if !m.isStreaming {
		return m, nil
	}
	m.isStreaming = false
	m.isReasoning = false
	m.forceHeaderRefresh = true
	// TODO: use chroma lexer to apply correct syntax highlighting to full response
	// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
	m.chat.AddResponse()
	curLineCount := m.viewport.TotalLineCount()

	// prepends the chat history to the screen
	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))

	if !m.preventScrollToBottom {
		m.viewport.GotoBottom()
	} else {
		// user has scrolled up during streaming. since we are now prepending the entire chat history before the latest response,
		// we need to set the Y offset so that their scroll position is the same as it was while streaming
		// TODO: may need to do this regardless
		yOffset := m.viewport.YOffset
		newLineCount := m.viewport.TotalLineCount()
		m.viewport.SetYOffset(newLineCount - curLineCount + yOffset)
	}
	m.preventScrollToBottom = false
	if !m.textarea.Focused() {
		// TODO: should check here that terminal has focus,
		// (user has changed windows since stream began)
		// otherwise Blink{} messages will continue to loop
		return m, m.textarea.Focus()
	}
	return m, nil
}

func (m *TUIModel) handleEscape() (tea.Model, tea.Cmd) {
	// m.viewport.GotoBottom()
	// m.lastManualGoToBottom = time.Now()
	if m.textarea.Focused() {
		if m.textarea.Length() > 0 && m.chat.HistoryLen() > 0 {
			m.textarea.Blur()
			// TODO: if height is > normal, set height to normal
			return m, m.redraw
		}
	} else if !m.isStreaming {
		return m, tea.Batch(m.textarea.Focus(), m.redraw)
		// if numLines > curHeight:
		// 		if numLines > normal, set height to min(numLines, maxHeight)
		// 		else set height to normal
	}
	return m, nil
}

func (m *TUIModel) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.chat.HistoryLen() == 0 {
		return m, tea.Quit
	}
	m.chat.Clear() // print something
	m.model.DoClearChatHistory()
	m.chat.Scrollback.Reset()
	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
	if !m.textarea.Focused() {
		return m, m.textarea.Focus()
	}
	return m, nil
}

func (m *TUIModel) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.chat.Scrollback.Reset()

	if input == "" {
		return m, nil
	}

	// Start LLM streaming
	return m.promptLLM(input)
}

// allowScrollback checks the cursor position in the textarea and returns whether triggering a scrollback action can take place
func (m *TUIModel) allowScrollback(keyString string) bool {
	realLineCount := m.textarea.LineCount() // # of lines given infinite screen width
	lineNo := m.textarea.Line() + 1         // starts at zero

	wrappedLineCount := m.getNumLines(m.textarea.Value()) // # of lines on screen incl. soft-wrapped

	li := m.textarea.LineInfo()
	cursorOnFirstRow := li.RowOffset == 0 // calculated according to soft-wrap
	cursorOnLastRow := lineNo == realLineCount

	// below are the conditions where we should let normal up/down cursor actions take place
	if cursorOnFirstRow && !cursorOnLastRow && wrappedLineCount > 1 && keyString == "down" {
		return false
	}
	if cursorOnLastRow && !cursorOnFirstRow && wrappedLineCount > 1 && keyString == "up" {
		return false
		// TODO: color the prompt lead differently on its first line?
	}
	if !cursorOnFirstRow && !cursorOnLastRow {
		// if cursor is somewhere in the middle of the text
		return false
	}
	// if the last line is soft-wrapped onto multiple terminal rows and
	// the cursor is not at the last row of that line
	if cursorOnFirstRow && li.Height > 1 && li.RowOffset > 0 {
		return false
	}
	// if the last line is soft-wrapped onto multiple terminal rows and
	// the cursor is not at the last row of that line
	if cursorOnLastRow && li.Height > 1 && li.RowOffset < li.Height-1 {
		return false
	}
	return true
}

// updateTextarea sends any message to the textarea. It also handles resizing the textarea if the text changes
func (m *TUIModel) updateTextarea(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg == nil {
		return m, nil
	}

	var (
		newHeight int
		taCmd     tea.Cmd
	)
	expanded, collapsed := styles.TEXTAREA_HEIGHT_NORMAL, styles.TEXTAREA_HEIGHT_COLLAPSED
	if m.textarea.Length() > 0 {
		if m.textarea.Height() < expanded {
			newHeight = expanded
		} else if numLines := m.getNumLines(m.textarea.Value()); numLines >= expanded {
			newHeight = math.Clamp(numLines, expanded, m.textarea.MaxHeight)
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
		return m.updateComponents(msg)
	}

	// This runs when the textarea is focused and not being resized.
	// NOTE: this prevents messages from reaching the viewport, which may not be desirable
	// ensure we aren't returning nil above these lines and therefore blocking messages to these models
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, taCmd
}

// triggerScrollback makes the textarea go forward or backward in history to display a different prompt
func (m *TUIModel) triggerScrollback(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var (
		retrievedPrompt string
		exists          bool
		taCmd           tea.Cmd
	)

	curPrompt := strings.TrimSpace(m.textarea.Value())
	if msg.String() == "up" {
		retrievedPrompt, exists = m.chat.Scrollback.PrevPrompt(curPrompt)
	} else {
		retrievedPrompt, exists = m.chat.Scrollback.NextPrompt(curPrompt)
		// TODO: set line number to 0 for better traversal
	}

	if !exists {
		return m, nil
	}

	m.textarea.SetValue(retrievedPrompt)
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, taCmd
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
func InitLLMClient(modelName, systemPrompt string, maxTokens int) (newModel models.LLM) {
	// var pastMessages []models.Message
	// if t.model != nil {
	// 	pastMessages = t.model.DoGetChatHistory()
	// }

	anthropicErr := anthropic.ValidateModelName(modelName)
	openAIErr := openai.ValidateModelName(modelName)

	switch {
	case anthropicErr != nil && openAIErr != nil:
		newModel = nil
	case anthropicErr != nil && openAIErr == nil:
		newModel = openai.NewModel(systemPrompt, maxTokens, modelName, nil)
	case openAIErr != nil && anthropicErr == nil:
		newModel = anthropic.NewModel(systemPrompt, maxTokens, modelName, nil)
	default:
		// This shouldn't happen if validation functions are implemented correctly
		newModel = nil
	}
	if newModel == nil {
		panic(fmt.Sprintf("Error initializing model:\nantErr: %v\nopenAIerr: %v", anthropicErr, openAIErr))
	}
	return newModel
}
