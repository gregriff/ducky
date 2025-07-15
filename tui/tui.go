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

type TUI struct {
	// styles *styles.TUIStylesStruct

	// user args TODO: combine these into a PromptContext struct (and add a context._), along with isStreaming + isReasoning?
	model           models.LLM
	systemPrompt    string
	maxTokens       int
	enableReasoning bool

	// UI state
	ready      bool
	ta         textarea.Model
	vp         viewport.Model
	spinner    spinner.Model
	windowSize tea.WindowSizeMsg

	lastLeftClick,
	lastManualGoToBottom time.Time
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
	ta.SetHeight(styles.TEXTAREA_HEIGHT_NORMAL)

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = styles.TUIStyles.Spinner

	t := &TUI{
		systemPrompt:    systemPrompt,
		maxTokens:       maxTokens,
		enableReasoning: enableReasoning,

		ta:      ta,
		spinner: s,

		chat:         chat.NewChatModel(glamourStyle),
		responseChan: make(chan models.StreamChunk),

		pagerTempfile: "temp.history",
	}

	t.initLLMClient(modelName)
	return t
}

func (t *TUI) Start() {
	p := tea.NewProgram(t,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		// tea.WithReportFocus(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

// Init performs initial IO.
func (t *TUI) Init() tea.Cmd {
	return tea.Batch(tea.SetWindowTitle("ducky"), textarea.Blink)
}

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd,
		spCmd,
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyString := msg.String()

		switch keyString {
		case "ctrl+d":
			return t, tea.Quit
		case "esc":
			t.vp.GotoBottom()
			t.lastManualGoToBottom = time.Now()
			if !t.ta.Focused() {
				t.ta.Focus()
			}
			t.ta.SetHeight(styles.TEXTAREA_HEIGHT_NORMAL)
			return t, func() tea.Msg {
				return t.windowSize
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
		if t.isStreaming {
			break
		}

		switch keyString {
		case "ctrl+c":
			if t.chat.HistoryLen() == 0 {
				return t, tea.Quit
			}
			t.chat.Clear() // print something
			t.vp.SetContent(t.chat.Render(t.vp.Width))
			return t, nil
		case "enter":
			input := strings.TrimSpace(t.ta.Value())
			t.ta.Reset()

			if input == "" {
				return t, nil
			}

			// Start LLM streaming
			return t.promptLLM(input)
		}

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			if t.isStreaming { // allow user to scroll up during streaming and keep their position
				t.preventScrollToBottom = true
			}

			// here we don't scroll up if the user has just pressed esc. On mac, the rapid scroll events build up, and may
			// register after the esc handler, which results in the viewport scrolling up after going to the bottom.
			if time.Since(t.lastManualGoToBottom) < 800*time.Millisecond {
				return t, nil
			}
			break
		}

		// the switch below will capture this button and prevent scroll so break out
		if msg.Button == tea.MouseButtonWheelDown {
			break
		}

		// handles all mouse EVENTS  TODO: re-evaluate for bugs
		switch msg.Action {
		case tea.MouseActionRelease:
			if t.isStreaming || msg.Button != tea.MouseButtonLeft {
				return t, nil
			}

			textareaFocused := t.ta.Focused()
			if zone.Get("chatViewport").InBounds(msg) {
				if t.chat.HistoryLen() == 0 {
					break
				}
				if textareaFocused {
					t.ta.Blur() // TODO: need to collapse it as well
				}
				if time.Since(t.lastLeftClick) < 300*time.Millisecond {
					selectedLine := max((msg.Y+t.vp.YOffset)-t.vp.Height/2, 0)                    // line of text user clicked
					err := os.WriteFile(t.pagerTempfile, []byte(t.chat.Render(t.vp.Width)), 0644) // should be its own tea.Msg?
					if err != nil {
						return t, func() tea.Msg {
							return pagerError{err: err}
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
						// "--color=PkY.EkY",   // set prompt and error color to black on bright yellow
						// fmt.Sprintf("--prompt=%s", `COPY MODE | %pb\% %BB ?m(response %i of %m).`), // (section %dt/%D)
						//
						// STATUS COLUMN: shows marks and matches in leftmost col
						// - width must be <= than H_PADDING or prompts will be truncated
						// - prompt and response strings would need to have their ending character removed
						//   to prevent less from showing truncation symbol
						// "--status-column",   // shows marks and matches
						// fmt.Sprintf("--status-col-width=%d", styles.H_PADDING),
						// "--save-marks", will need this later
						t.pagerTempfile,
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
					return t, tea.ExecProcess(cmd, onPagerExit)
				} else {
					t.lastLeftClick = time.Now()
				}

			} else if zone.Get("promptInput").InBounds(msg) {
				if !textareaFocused {
					t.ta.Focus()
				}
			}
		}

	case models.StreamChunk:
		t.isReasoning = msg.Reasoning
		t.chat.AccumulateStream(msg.Content, msg.Reasoning, false)

		t.vp.SetContent(t.chat.Render(t.vp.Width))
		if !t.preventScrollToBottom {
			t.vp.GotoBottom()
		}
		return t, t.waitForNextChunk

	// TODO: include usage data by having DoStreamPromptCompletion return this with fields?
	case streamComplete: // responseChan guaranteed to be empty here
		// if a StreamError occurs before response streaming begins, two waitForNextChunks will return streamComplete
		if t.isStreaming == false {
			return t, nil
		}
		t.isStreaming = false
		t.isReasoning = false
		// t.ta.SetHeight(styles.TEXTAREA_HEIGHT_NORMAL)
		// TODO: use chroma lexer to apply correct syntax highlighting to full response
		// lexer := lexers.Analyse("package main\n\nfunc main()\n{\n}\n")
		t.chat.AddResponse()

		t.vp.SetContent(t.chat.Render(t.vp.Width))

		if !t.preventScrollToBottom {
			t.vp.GotoBottom()
		}
		t.preventScrollToBottom = false
		if !t.ta.Focused() {
			t.ta.Focus()
		}

		// recalculate views because we've changed the textarea height
		// resizeWindow := func() tea.Msg {
		// 	return t.windowSize
		// }

		return t, textarea.Blink

	case models.StreamError:
		errMsg := fmt.Sprintf("**Error:** %v", msg.ErrMsg)
		t.chat.AccumulateStream(errMsg, false, true)
		return t, t.waitForNextChunk // ensure last chunk is read and let chunk and complete messages handle state

	case pagerExit:
		// pager lets term control mouse for selecting/copying. Regain those controls and fullscreen
		return t, tea.Batch(tea.EnterAltScreen, tea.EnableMouseCellMotion, t.removeTempFile)

	case pagerError:
		pagerErr := msg.err.Error()
		if pagerErr != "exit status 2" {
			t.ta.InsertString(fmt.Sprintf("Pager Error: %s\n", pagerErr))
		}
		return t, tea.Batch(tea.EnterAltScreen, tea.EnableMouseCellMotion, t.removeTempFile)

	case tea.WindowSizeMsg:
		t.windowSize = msg

		headerHeight := lipgloss.Height(t.headerView())
		textAreaHeight := t.ta.Height()
		verticalMarginHeight := headerHeight + textAreaHeight + 1 // +1 for spacing in View()

		viewportHeight := msg.Height - verticalMarginHeight
		textAreaWidth := msg.Width - styles.H_PADDING
		markdownWidth := int(float64(msg.Width) * styles.WIDTH_PROPORTION_RESPONSE)

		// TODO: should be able to move this into constructor, and style Viewport with vp.Style
		if !t.ready {
			t.vp = viewport.New(msg.Width, viewportHeight)
			t.vp.MouseWheelDelta = 2
			t.chat.Markdown.SetWidth(markdownWidth)
			t.vp.SetContent(t.chat.Render(msg.Width))
			t.vp.GotoBottom()
			t.ta.SetWidth(textAreaWidth)
			t.ready = true
		} else {
			t.vp.Width = msg.Width
			t.vp.Height = viewportHeight

			t.ta.SetWidth(textAreaWidth)
			t.chat.Markdown.SetWidth(msg.Width)
			t.vp.SetContent(t.chat.Render(msg.Width))
		}
	}

	// ensure we aren't returning nil above these lines and therefore blocking messages to these models
	t.ta, taCmd = t.ta.Update(msg)

	// this will be used if we change the height below
	taResizeCmds := tea.Batch(taCmd, textarea.Blink, func() tea.Msg {
		return t.windowSize
	})

	// this expands the textarea if user starts typing and collapses it if they clear it
	if t.ta.Length() > 0 {
		if t.ta.Height() < styles.TEXTAREA_HEIGHT_NORMAL {
			t.ta.SetHeight(styles.TEXTAREA_HEIGHT_NORMAL)
			taCmd = taResizeCmds
		}
	} else if t.ta.Height() > styles.TEXTAREA_HEIGHT_COLLAPSED {
		t.ta.SetHeight(styles.TEXTAREA_HEIGHT_COLLAPSED)
		taCmd = taResizeCmds
	}

	t.spinner, spCmd = t.spinner.Update(msg)

	// prevent movement keys from scrolling the viewport
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "d", "u", "b", "j", "k":
			break
		}
	default:
		t.vp, vpCmd = t.vp.Update(msg)
	}
	return t, tea.Batch(taCmd, vpCmd, spCmd)
}

func (t *TUI) removeTempFile() tea.Msg {
	err := os.Remove(t.pagerTempfile)
	if err != nil {
		t.ta.InsertString(fmt.Sprintf("Error deleting tempfile: %e\n", err))
	}
	return nil
}

// promptLLM makes the LLM API request, handles TUI state and begins listening for the response stream
func (t *TUI) promptLLM(prompt string) (tea.Model, tea.Cmd) {
	t.responseChan = make(chan models.StreamChunk)
	t.isStreaming = true
	if t.enableReasoning { // TODO: && model.supportsReasoning (make new interface func)
		t.isReasoning = true
	}

	if t.ta.Focused() {
		t.ta.Blur()
	}
	t.ta.SetHeight(styles.TEXTAREA_HEIGHT_COLLAPSED)

	t.chat.AddPrompt(prompt)
	t.vp.SetContent(t.chat.Render(t.vp.Width))
	t.vp.GotoBottom()

	// recalculate view because we've changed the textarea height
	triggerResize := func() tea.Msg {
		return t.windowSize
	}

	beginStreaming := func() tea.Msg {
		return models.StreamPromptCompletion(t.model, prompt, t.enableReasoning, t.responseChan)
	}

	return t, tea.Batch(
		t.spinner.Tick,
		triggerResize,
		beginStreaming,
		t.waitForNextChunk,
	)
}

// waitForNextChunk notifies the Update function when a response chunk arrives, and also when the response is completed.
func (t *TUI) waitForNextChunk() tea.Msg {
	if chunk, ok := <-t.responseChan; ok {
		return chunk
	} else {
		return streamComplete{}
	}

}

func (t *TUI) View() string {
	if !t.ready {
		return "Initializing..."
	}
	// NOTE: if you change this you need to change code in the window resize event handler
	spacing := "\n"
	return zone.Scan(
		fmt.Sprintf("%s\n%s\n%s", // NOTE: newlines needed between every component placed vertically (so they're not sidebyside and wrapped)
			t.headerView(),
			zone.Mark("chatViewport", t.vp.View()),
			zone.Mark("promptInput", spacing+t.ta.View()),
		),
	)
}

func (t *TUI) headerView() string {
	var leftText string
	if t.isStreaming {
		leftText = t.spinner.View()
	} else {
		leftText = "ducky"
	}
	rightText := models.GetModelId(t.model)
	maxWidth := t.vp.Width - styles.HEADER_R_PADDING
	titleTextWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText) + 2 // the two border chars
	spacing := strings.Repeat(" ", max(5, maxWidth-titleTextWidth))

	return styles.TUIStyles.TitleBar.Width(max(0, maxWidth)).
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
