package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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

	lastLeftClick time.Time
	// lastManualGoToBottom time.Time
	pagerTempfile string

	// Chat state
	chat *chat.ChatModel
	isStreaming,
	isReasoning bool
	responseChan chan models.StreamChunk

	preventScrollToBottom bool
}

// Bubbletea messages
type streamComplete struct{}

type pagerExit struct{}
type pagerError struct{ err error }

func NewTUI(systemPrompt string, modelName string, enableReasoning bool, maxTokens int, glamourStyle string) *TUIModel {
	// create and style textarea
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // TODO: need this to be bound to shift+enter
	ta.Placeholder = "Send a prompt..."
	ta.FocusedStyle.Placeholder = styles.TUIStyles.PromptText
	ta.FocusedStyle.CursorLine = styles.TUIStyles.TextAreaCursor
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

	t.initLLMClient(modelName)
	return t
}

func (m *TUIModel) Start() {
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		// tea.WithReportFocus(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

// Init performs initial IO.
func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(tea.SetWindowTitle("ducky"), textarea.Blink, m.textarea.Focus())
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd,
		spCmd,
		vpCmd tea.Cmd

		cmds []tea.Cmd
	)

msgType:
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
				cmds = append(cmds, m.textarea.Focus(), textarea.Blink, m.redraw)
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

		switch keyString {
		case "ctrl+c":
			if m.chat.HistoryLen() == 0 {
				return m, tea.Quit
			}
			m.chat.Clear() // print something
			m.viewport.SetContent(m.chat.Render(m.viewport.Width))
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

	case tea.MouseMsg:
		// Here we define a condition where the textarea can be focused, but scroll events will be sent to the viewport instead.
		sendScrollToViewport := m.textarea.Focused() && m.textarea.LineCount() <= m.textarea.Height()

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			// here we don't scroll up if the user has just pressed esc. On mac, the rapid scroll events build up, and may
			// register after the esc handler, which results in the viewport scrolling up after going to the bottom.
			// if time.Since(m.lastManualGoToBottom) < 800*time.Millisecond {
			// return m, nil
			// }
			if m.isStreaming { // allow user to scroll up during streaming and keep their position
				m.preventScrollToBottom = true
			}
			fallthrough // I know this and the labeled break are ugly but I'm experimenting here
		case tea.MouseButtonWheelDown:
			var scrollCmd tea.Cmd
			if sendScrollToViewport {
				m.viewport, scrollCmd = m.viewport.Update(msg)
				return m, scrollCmd
			}
			// break out of the top-level switch because we still need the normal viewport.Update to handle the scroll events
			// (this break allows the viewport to scroll when the textinput is unfocused and m.textarea.LineCount() <= m.textarea.Height())
			break msgType
		}

		// handles all mouse EVENTS  TODO: re-evaluate for bugs
		switch msg.Action {
		case tea.MouseActionRelease:
			if m.isStreaming || msg.Button != tea.MouseButtonLeft {
				return m, nil
			}

			textareaFocused := m.textarea.Focused()
			if zone.Get("chatViewport").InBounds(msg) {
				if m.chat.HistoryLen() == 0 {
					break
				}
				if textareaFocused {
					m.textarea.Blur() // TODO: need to collapse it as well
				}
				if time.Since(m.lastLeftClick) < 300*time.Millisecond {
					tmpFile, err := os.CreateTemp(".", "pager-*")
					if err != nil {
						return m, func() tea.Msg {
							return pagerError{err: err}
						}
					}
					m.pagerTempfile = tmpFile.Name()

					// calculate the line Number clicked to open the pager in the exact same position as what is on screen
					selectedLine := max(m.viewport.YOffset-2, 0) // I don't know where the 2 comes from
					_, writeErr := tmpFile.WriteString(m.chat.Render(m.viewport.Width))
					if writeErr != nil {
						return m, func() tea.Msg {
							return pagerError{err: writeErr}
						}
					}
					cmd := exec.Command(
						"less",
						fmt.Sprintf("+%d", selectedLine),
						"--use-color",       // display ANSI colors
						"--chop-long-lines", // dont wrap long lines
						"--quit-on-intr",    // quit on ctrl+c
						"--incsearch",       // incremental search
						fmt.Sprintf("--prompt=%s", `?eEOF ?m(response %i of %m).`),
						m.pagerTempfile,
					)
					cmd.Env = append(os.Environ(),
						"LESSSECURE=1", // disables in-pager shell, editing, pipe etc.
					)
					onPagerExit := func(err error) tea.Msg {
						if err != nil {
							return pagerError{err: err}
						}
						return pagerExit{}
					}
					return m, tea.ExecProcess(cmd, onPagerExit)
				} else {
					m.lastLeftClick = time.Now()
					// cmds = append(cmds, m.redraw)
				}

			} else if zone.Get("promptInput").InBounds(msg) {
				if !textareaFocused {
					cmds = append(cmds, m.textarea.Focus(), m.redraw)
				}
			}
		}

	case models.StreamChunk:
		m.isReasoning = msg.Reasoning
		m.chat.AccumulateStream(msg.Content, msg.Reasoning, false)

		m.viewport.SetContent(m.chat.Render(m.viewport.Width))
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
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		m.chat.AddResponse()

		m.viewport.SetContent(m.chat.Render(m.viewport.Width))

		if !m.preventScrollToBottom {
			m.viewport.GotoBottom()
		}
		m.preventScrollToBottom = false
		if !m.textarea.Focused() {
			cmds = append(cmds, m.textarea.Focus())
		}

		return m, textarea.Blink

	case models.StreamError:
		errMsg := fmt.Sprintf("**Error:** %v", msg.ErrMsg)
		m.chat.AccumulateStream(errMsg, false, true)
		return m, m.waitForNextChunk // ensure last chunk is read and let chunk and complete messages handle state

	case spinner.TickMsg:
		if m.isStreaming {
			m.spinner, spCmd = m.spinner.Update(msg)
		}
		return m, spCmd

	case pagerExit:
		// pager lets term control mouse for selecting/copying. Regain those controls and fullscreen
		return m, tea.Batch(tea.EnterAltScreen, tea.EnableMouseCellMotion, m.removeTempFile)

	case pagerError:
		pagerErr := msg.err.Error()
		if pagerErr != "exit status 2" {
			m.textarea.InsertString(fmt.Sprintf("Pager Error: %s\n", pagerErr))
		}
		return m, tea.Batch(tea.EnterAltScreen, tea.EnableMouseCellMotion, m.removeTempFile)

	case tea.WindowSizeMsg:
		m.windowSize = msg

		headerHeight := lipgloss.Height(m.headerView())
		textAreaHeight := m.textarea.Height()
		verticalMarginHeight := headerHeight + textAreaHeight + 1 // +1 for spacing in View()

		viewportHeight := msg.Height - verticalMarginHeight
		textAreaWidth := msg.Width - styles.H_PADDING
		markdownWidth := int(float64(msg.Width) * styles.WIDTH_PROPORTION_RESPONSE)

		// TODO: should be able to move this into constructor, and style Viewport with vp.Style
		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.MouseWheelDelta = 2
			m.chat.Markdown.SetWidth(markdownWidth)
			m.viewport.SetContent(m.chat.Render(msg.Width))
			m.viewport.GotoBottom()
			m.textarea.SetWidth(textAreaWidth)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight

			m.textarea.SetWidth(textAreaWidth)
			m.chat.Markdown.SetWidth(msg.Width)
			m.viewport.SetContent(m.chat.Render(msg.Width))
		}
		// m.textarea.MaxHeight = viewportHeight / 2
	}

	if m.textarea.Focused() {
		// ensure we aren't returning nil above these lines and therefore blocking messages to these models
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)

		// this expands the textarea if user starts typing and collapses it if they clear it
		expanded, collapsed := styles.TEXTAREA_HEIGHT_NORMAL, styles.TEXTAREA_HEIGHT_COLLAPSED
		if m.textarea.Length() > 0 {
			if m.textarea.Height() < expanded {
				m.textarea.SetHeight(expanded)
				cmds = append(cmds, m.redraw, textarea.Blink)
			}
		} else if m.textarea.Height() > collapsed {
			m.textarea.SetHeight(collapsed)
			cmds = append(cmds, m.redraw, textarea.Blink)
		}
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

// redraw initiates the Window resize handler. Use it after changing the dimensions of a component to make the others update
func (m *TUIModel) redraw() tea.Msg {
	return m.windowSize
}

func (m *TUIModel) removeTempFile() tea.Msg {
	err := os.Remove(m.pagerTempfile)
	if err != nil {
		m.textarea.InsertString(fmt.Sprintf("Error deleting tempfile: %e\n", err))
	}
	return nil
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream
func (m *TUIModel) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	m.responseChan = make(chan models.StreamChunk)
	m.isStreaming = true
	if m.enableReasoning { // TODO: && model.supportsReasoning (make new interface func)
		m.isReasoning = true
	}

	if m.textarea.Focused() {
		m.textarea.Blur()
	}

	m.chat.AddPrompt(prompt)
	m.viewport.SetContent(m.chat.Render(m.viewport.Width))
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
	// NOTE: if you change this you need to change code in the window resize event handler
	spacing := "\n"
	return zone.Scan(
		fmt.Sprintf("%s\n%s\n%s", // NOTE: newlines needed between every component placed vertically (so they're not sidebyside and wrapped)
			m.headerView(),
			zone.Mark("chatViewport", m.viewport.View()),
			zone.Mark("promptInput", spacing+m.textarea.View()),
		),
	)
}

func (m *TUIModel) headerView() string {
	var leftText string
	if m.isStreaming {
		leftText = m.spinner.View()
	} else {
		leftText = "ducky"
	}
	rightText := models.GetModelId(m.model)
	maxWidth := m.viewport.Width - styles.HEADER_R_PADDING
	titleTextWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText) + 2 // the two border chars
	spacing := strings.Repeat(" ", max(5, maxWidth-titleTextWidth))

	return styles.TUIStyles.TitleBar.Width(max(0, maxWidth)).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, leftText, spacing, rightText))

}

// initLLMClient creates an LLM Client given a modelName. It is called at TUI init, and can be called any time later
// in order to switch between LLMs while preserving message history
func (m *TUIModel) initLLMClient(modelName string) error {
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
			m.model = provider.newModelFunc(m.systemPrompt, m.maxTokens, modelName, nil)
			return nil
		}
	}

	return fmt.Errorf("unsupported model: %s", modelName)
}
