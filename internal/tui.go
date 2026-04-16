// Package internal contains the code needed to run a ducky TUI app
package internal

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gregriff/ducky/internal/chat"
	"github.com/gregriff/ducky/internal/models"
	"github.com/gregriff/ducky/internal/models/anthropic"
	"github.com/gregriff/ducky/internal/models/openai"
	styles "github.com/gregriff/ducky/internal/styles"
	"github.com/muesli/reflow/wordwrap"
)

// model defines the TUI application state.
type model struct {
	// user args TODO: combine these into a PromptContext struct (and add a context._), along with isStreaming + isReasoning?
	llm             models.LLM
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
	chat *chat.Model
	isStreaming,
	isReasoning bool
	responseChan chan models.StreamChunk

	preventScrollToBottom bool

	// rendering
	contentBuilder,
	headerBuilder strings.Builder
	lastWidth          int
	forceHeaderRefresh bool

	streamCtx     context.Context
	stopStreaming context.CancelFunc
}

// Bubbletea messages.
type (
	makeInitialPrompt struct{}
	streamComplete    struct{}
)

// NewTUI creates the TUI application with default state.
func NewTUI(
	llm models.LLM,
	systemPrompt string,
	enableReasoning bool,
	reasoningEffort *uint8,
	maxTokens int,
	glamourStyle string,
) *model {
	// create and style textarea
	ta := textarea.New()
	ta.DynamicHeight = true
	ta.MinHeight = styles.TA_HEIGHT_COLLAPSED // Minimum visible rows
	ta.MaxHeight = styles.TA_HEIGHT_NORMAL    // Maximum visible rows
	ta.MaxContentHeight = 1000                // Maximum rows of content

	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // TODO: need this to be bound to shift+enter
	ta.Placeholder = "Send a prompt..."

	taS := ta.Styles()
	taS.Focused.Placeholder = styles.TUIStyles.PromptText
	taS.Focused.CursorLine = styles.TUIStyles.TextAreaCursor
	ta.SetStyles(taS)

	ta.Prompt = "┃ "
	ta.CharLimit = 100_000
	ta.SetHeight(styles.TA_HEIGHT_NORMAL)

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = styles.TUIStyles.Spinner

	return &model{
		systemPrompt:    systemPrompt,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,
		reasoningEffort: reasoningEffort,
		llm:             llm,

		textarea: ta,
		spinner:  s,

		chat:         chat.NewChatModel(glamourStyle),
		responseChan: make(chan models.StreamChunk),
	}
}

// Start begins the TUI application.
func (m *model) Start(initialPrompt string) {
	p := tea.NewProgram(m)
	m.initialPrompt = initialPrompt
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

// Init performs initial IO.
func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.textarea.Focus()}

	if len(m.initialPrompt) > 0 {
		cmds = append(cmds, func() tea.Msg {
			return makeInitialPrompt{}
		})
	}
	return tea.Batch(cmds...)
}

// Update updates the TUI UI.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var spCmd, vpCmd tea.Cmd

	// log.Printf("MESSAGE RECEIVED: %#v\n", msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyString := msg.String()

		switch keyString {
		case "ctrl+d":
			return m, tea.Quit
		case "ctrl+c":
			return m.handleCtrlC()
		case "ctrl+l":
			return m.handleCtrlL()
		case "esc":
			return m.handleEscape()
		case "up", "down":
			// if not allowed, arrow key inputs will be handled by the textarea if its focused
			if allow := m.allowScrollback(keyString); !allow {
				break
			}
			return m.triggerScrollback(msg)
		}

		// while streaming, anything below this will not be accessible
		if m.isStreaming {
			break
		}

		switch keyString {
		case "enter":
			return m.handleEnter()
		}
	// case tea.PasteStartMsg:
	// case tea.PasteEndMsg:
	case tea.PasteMsg:
		if m.isStreaming { // don't allow paste while streaming
			return m, nil
		}
		// paste will be passed to textarea
	case tea.MouseMsg:
		switch msg := msg.(type) {
		case tea.MouseClickMsg:
			return m.handleClick(msg)
		case tea.MouseReleaseMsg:
			return m, nil
		case tea.MouseWheelMsg:
			return m.handleScroll(msg)
		}

	case tea.BlurMsg:
		if m.textarea.Focused() {
			m.textarea.Blur()
		}
		return m, nil

	// NOTE: on tmux, regaining focus from switching panes results in
	// `tea.unknownCSISequenceMsg{0x1b, 0x5b, 0x49}`, so this is not run
	case tea.FocusMsg:
		return m, m.textarea.Focus()

	case makeInitialPrompt:
		return m.promptLLM(m.initialPrompt)

	case models.StreamChunk:
		return m.handleStreamChunk(msg)

	// TODO: include usage data by having DoStreamPromptCompletion return this with fields?
	case streamComplete: // responseChan guaranteed to be empty here
		return m.handleStreamComplete()

	case models.StreamError:
		return m.handleStreamError(msg)

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
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.Key().Text {
		case "d", "u", "b", "j", "k",
			"ctrl+d", "ctrl+u", "ctrl+b", "ctrl+j", "ctrl+k":
			return m, nil
		}
	}

	// ignore keys that would move the viewport by a single line
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "ctrl+d", "ctrl+u", "ctrl+b", "ctrl+j", "ctrl+k":
			return m, nil
		}
	}

	if _, ok := msg.(cursor.BlinkMsg); ok {
		return m, nil
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// redraw initiates the Window resize handler. Use it after changing the dimensions of a component to make the others update.
func (m *model) redraw() tea.Msg {
	return m.windowSize
}

// resizeComponents sets size properties on the viewport and textarea.
func (m *model) resizeComponents(windowWidth, textAreaWidth, viewportHeight int) {
	m.viewport.SetWidth(windowWidth)
	m.viewport.SetHeight(viewportHeight)

	m.textarea.MaxWidth = textAreaWidth
	m.textarea.SetWidth(textAreaWidth)

	m.viewport.SetContent(m.chat.Render(windowWidth))
}

// getResizeParams returns size dimensions of on-screen components needed during redrawing or resizing.
func (m *model) getResizeParams(windowHeight, windowWidth int) (viewportHeight int, textAreaWidth int) {
	headerHeight := lipgloss.Height(m.headerView())
	verticalMarginHeight := headerHeight + m.textarea.Height() + styles.VP_TA_SPACING_SIZE

	viewportHeight = windowHeight - verticalMarginHeight
	textAreaWidth = windowWidth - styles.H_PADDING
	return viewportHeight, textAreaWidth
}

func (m *model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.windowSize = msg
	windowHeight, windowWidth := msg.Height, msg.Width
	viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth)

	var taCmd, vpCmd tea.Cmd
	// TODO: should be able to move this into constructor, and style Viewport with vp.Style
	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(windowWidth), (viewport.WithHeight(viewportHeight)))
		m.viewport.MouseWheelDelta = 2 // TODO: make this configurable
		m.viewport.SetContent(m.chat.Render(windowWidth))
		m.viewport.GotoBottom()
		// vp.LeftGutterFunc = func(info viewport.GutterContext) ...
		m.textarea.MaxWidth = textAreaWidth
		m.textarea.SetWidth(textAreaWidth)
		m.textarea.MaxHeight = viewportHeight / 2
		m.ready = true

		m.viewport, vpCmd = m.viewport.Update(msg)
		m.textarea, taCmd = m.textarea.Update(msg)
		return m, tea.Batch(taCmd, vpCmd)
	}

	m.textarea.MaxHeight = viewportHeight / 2

	m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, tea.Batch(taCmd, vpCmd)
}

// getNumLines returns the number of lines the text in the textarea takes up (soft-wrapped).
// takes into account lines of text not visible on screen (scrolled out of view).
func (m *model) getNumLines(text string) int {
	wrapped := wordwrap.String(text, m.textarea.MaxWidth)
	lines := strings.Split(wrapped, "\n")
	return len(lines)
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream.
func (m *model) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	m.responseChan = make(chan models.StreamChunk)
	m.isStreaming = true
	if m.enableReasoning && m.llm.SupportsReasoning() {
		m.isReasoning = true
	}

	if m.textarea.Focused() {
		m.textarea.Blur()
	}

	m.chat.AddPrompt(prompt)
	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
	m.viewport.GotoBottom()

	m.streamCtx, m.stopStreaming = context.WithCancel(context.Background())
	beginStreaming := func() tea.Msg {
		err := m.llm.StreamPromptCompletion(m.streamCtx, prompt, m.enableReasoning, m.reasoningEffort, m.responseChan)
		if err != nil {
			return models.StreamError{ErrMsg: err.Error()}
		}
		return nil
	}

	return m, tea.Batch(
		m.spinner.Tick,
		m.redraw, // recalculate view because we've changed the textarea height
		beginStreaming,
		m.waitForNextChunk,
	)
}

func (m *model) handleStreamChunk(msg models.StreamChunk) (tea.Model, tea.Cmd) {
	m.isReasoning = msg.Reasoning
	m.chat.AccumulateStream(msg.Content, msg.Reasoning, false)

	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
	if !m.preventScrollToBottom {
		m.viewport.GotoBottom()
	}
	return m, m.waitForNextChunk
}

// waitForNextChunk notifies the Update function when a response chunk arrives, and also when the response is completed.
func (m *model) waitForNextChunk() tea.Msg {
	select {
	case <-m.streamCtx.Done():
		return models.StreamError{ErrMsg: m.streamCtx.Err().Error()}
	case chunk, ok := <-m.responseChan:
		if !ok {
			return streamComplete{}
		}
		return chunk
	}
}

// handleStreamComplete updates TUI state when a LLM response has been fully received.
func (m *model) handleStreamComplete() (tea.Model, tea.Cmd) {
	// if a StreamError occurs before response streaming begins, two waitForNextChunks will return streamComplete
	if !m.isStreaming {
		return m, nil
	}
	m.isStreaming = false
	m.isReasoning = false
	m.forceHeaderRefresh = true

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
		yOffset := m.viewport.YOffset()
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

func (m *model) handleStreamError(msg models.StreamError) (tea.Model, tea.Cmd) {
	var errMsg string

	// if stream has not been canceled and we indeed have an error
	if m.streamCtx.Err() == nil {
		errMsg = fmt.Sprintf("**Error:** %v", msg.ErrMsg)
	} else {
		// TODO: after 1 cancel, any subsequent error will probably show this message. reset the Context
		errMsg = ">Stream Cancelled"
	}

	m.chat.AccumulateStream(errMsg, false, true)
	return m, m.waitForNextChunk // ensure last chunk is read and let chunk and complete messages handle state
}

func (m *model) handleScroll(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	var (
		scrollCmd     tea.Cmd
		scrollKey     tea.KeyMsg
		triggerScroll bool
		mouse         = msg.Mouse()
	)
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
		return m, nil
	}

	if m.textarea.Focused() {
		// TODO: cache this with sha1, scrolling large TA is laggy.
		wrappedLineCount := m.getNumLines(m.textarea.Value())
		taHeight := m.textarea.Height()

		// if there is not enough text in the TA to scroll it, OR
		// the TA is collapsed, scroll the viewport.
		// NOTE: this may be responsible for text flickering in the VP.
		if wrappedLineCount < taHeight || taHeight == styles.TA_HEIGHT_COLLAPSED {
			// NOTE: why do we not pass scrollKey here?
			m.viewport, scrollCmd = m.viewport.Update(msg)
		} else {
			m.textarea, scrollCmd = m.textarea.Update(scrollKey)
		}
	} else {
		// TODO: calculate YOffset and unrender/rerender chat history by:
		// m.viewport.SetContent(m.chat.Render()) <- add YOffset param
		// - only run SetContent if YOffset has just entered a new chatentry bounds
		// NOTE: YOffset will change when you decrease the amount of text in the viewport
		m.viewport, scrollCmd = m.viewport.Update(msg)
	}
	return m, scrollCmd
}

func (m *model) handleEscape() (tea.Model, tea.Cmd) {
	// m.viewport.GotoBottom()
	// m.lastManualGoToBottom = time.Now()
	if m.textarea.Focused() {
		if m.textarea.Length() > 0 && m.chat.HistoryLen() > 0 {
			m.textarea.Blur()
			// TODO: if height is > normal, set height to normal
			return m, m.redraw
		}
	} else if !m.isStreaming {
		return m, tea.Sequence(m.textarea.Focus(), m.redraw)
		// if numLines > curHeight:
		// 		if numLines > normal, set height to min(numLines, maxHeight)
		// 		else set height to normal
	}
	return m, nil
}

func (m *model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.isStreaming {
		m.stopStreaming()
		return m, nil
	}

	if m.chat.HistoryLen() == 0 {
		return m, tea.Quit
	}
	return m.clearChatHistory()
}

func (m *model) handleCtrlL() (tea.Model, tea.Cmd) {
	if m.isStreaming || m.chat.HistoryLen() == 0 {
		return m, nil
	}
	return m.clearChatHistory()
}

func (m *model) clearChatHistory() (tea.Model, tea.Cmd) {
	m.chat.Clear() // print something
	m.llm.ClearChatHistory()
	m.forceHeaderRefresh = true
	m.chat.Scrollback.Reset()
	m.viewport.SetContent(m.chat.Render(m.viewport.Width()))
	if !m.textarea.Focused() {
		return m, m.textarea.Focus()
	}
	return m, nil
}

func (m *model) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.chat.Scrollback.Reset()

	if input == "" {
		return m, nil
	}

	// Start LLM streaming
	return m.promptLLM(input)
}

// handleClick handles left mouse clicks.
// TODO: handle right clicks
func (m *model) handleClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	var vpCmd tea.Cmd
	if m.isStreaming || msg.Button != tea.MouseLeft {
		return m, nil
	}

	// NOTE: should refactor this logic.
	// TODO: clicking title bar is not handled

	// if the click is inside the viewport
	if msg.Y < m.windowSize.Height-m.textarea.Height() {
		if m.chat.HistoryLen() == 0 {
			return m, nil
		}
		// this allows the user to click the viewport and not have the
		// textarea be unfocused if theres not a lot of text in it
		if m.textarea.Focused() && m.getNumLines(m.textarea.Value()) > styles.TA_HEIGHT_COLLAPSED {
			m.textarea.Blur() // NOTE: could collapse it as well
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}

		// user has clicked the viewport and TA is not focused.
		if !m.textarea.Focused() {
			return m, nil
		}
	}

	if m.textarea.Focused() {
		return m, nil // don't send click if its already selected
	}
	return m, m.textarea.Focus()
}

// allowScrollback checks the cursor position in the textarea and returns whether triggering a scrollback action can take place.
func (m *model) allowScrollback(keyString string) bool {
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

// updateTextarea sends any message to the textarea.
func (m *model) updateTextarea(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg == nil {
		return m, nil
	}
	var taCmd tea.Cmd

	// TODO: this redraw block needs to be prevented for most msgs.
	// figure out which msgs require it (resize only) and whitelist that
	// instead of blacklisting.
	lastTaHeight := m.textarea.Height()

	// This runs when the textarea is focused and not being resized.
	// NOTE: this prevents messages from reaching the viewport, which may not be desirable
	// ensure we aren't returning nil above these lines and therefore blocking messages to these models
	m.textarea, taCmd = m.textarea.Update(msg)

	if m.textarea.Height() != lastTaHeight {
		windowHeight, windowWidth := m.windowSize.Height, m.windowSize.Width
		viewportHeight, textAreaWidth := m.getResizeParams(windowHeight, windowWidth)
		m.resizeComponents(windowWidth, textAreaWidth, viewportHeight)
	}
	return m, taCmd
}

// triggerScrollback makes the textarea go forward or backward in history to display a different prompt.
func (m *model) triggerScrollback(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

// View renders the TUI into a string.
func (m *model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = true
	v.WindowTitle = "ducky"

	if !m.ready {
		v.SetContent("Initializing...")
		return v
	}

	m.contentBuilder.Reset()
	m.contentBuilder.WriteString(m.headerView())
	m.contentBuilder.WriteString("\n")
	m.contentBuilder.WriteString(m.viewport.View())
	m.contentBuilder.WriteString("\n")
	m.contentBuilder.WriteString(styles.VP_TA_SPACING)
	m.contentBuilder.WriteString(m.textarea.View())
	v.SetContent(m.contentBuilder.String())
	return v
}

// headerView returns the formatted header, reusing the last computed headerView result if the width hasn't changed and the spinner doesn't
// need to be updated.
func (m *model) headerView() string {
	var leftText string
	width := m.viewport.Width()

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

	rightText := m.llm.ModelInfoText()
	if cost := models.FormattedCost(m.llm); cost != "" {
		rightText += " (" + cost + ")"
	}
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
// in order to switch between LLMs while preserving message history.
func InitLLMClient(modelName, systemPrompt string, maxTokens int, bedrockConfig *models.BedrockConfig) (newModel models.LLM, err error) {
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
		newModel, err = anthropic.NewModel(systemPrompt, maxTokens, modelName, nil, bedrockConfig)
	default:
		// This shouldn't happen if validation functions are implemented correctly
		newModel = nil
	}
	if newModel == nil {
		err = fmt.Errorf("error initializing model:\nanthropicError: %w\nopenAIError:%w", anthropicErr, openAIErr)
	}
	return newModel, err
}
