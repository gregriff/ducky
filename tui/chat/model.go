package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
)

// ChatModel stores the state of the current chat with the LLM and formats prompts/responses
type ChatModel struct {
	styles *styles.ChatStylesStruct

	history         []ChatEntry
	CurrentResponse *CurrentResponse
	TotalCost       float64

	renderedHistory  strings.Builder // stores accumulated chat history rendered in markdown and color for a specific width
	Markdown         *MarkdownRenderer
	currentWrapWidth int // # of term columns the stored prompts are word-wrapped to fit into
	numChatsRendered int
}

type CurrentResponse struct {
	ReasoningContent strings.Builder
	ResponseContent  strings.Builder
	ErrorContent     string
}

// Len returns the total byte count of the resoning and response parts of the current response
func (res *CurrentResponse) Len() int {
	return res.ReasoningContent.Len() + res.ResponseContent.Len()
}

func NewChatModel(glamourStyle string) *ChatModel {
	return &ChatModel{
		styles: &styles.ChatStyles,

		CurrentResponse: &CurrentResponse{},
		history:         make([]ChatEntry, 0, 10),
		Markdown:        NewMarkdownRenderer(glamourStyle),
	}
}

func (c *ChatModel) numPrompts() int {
	return len(c.history) // prompts determine the creation of new chat entries
}

func (c *ChatModel) numResponses() int {
	totalResponses := 0
	for _, entry := range c.history {
		if len(entry.response) > 0 {
			totalResponses += 1
		}
	}
	return totalResponses
}

// AddPrompt creates a new ChatEntry with prompt data
func (c *ChatModel) AddPrompt(s string) {
	newEntry := &ChatEntry{prompt: s}
	c.history = append(c.history, *newEntry)
}

// AddResponse updates the latest ChatEntry with the data from CurrentResponse. Must be called after AddPrompt
func (c *ChatModel) AddResponse() {
	s := c.CurrentResponse

	curChatEntry := &c.history[len(c.history)-1]
	curChatEntry.reasoning = s.ReasoningContent.String()
	curChatEntry.response = s.ResponseContent.String()
	curChatEntry.error = s.ErrorContent

	s.ReasoningContent.Reset()
	s.ResponseContent.Reset()
	s.ErrorContent = ""
}

// Render returns a string of the entire chat history in markdown, wrapped to a certain width
func (c *ChatModel) Render(vpWidth int) string {
	// TODO: only reset if width has changed (pass a width) and keep a copy of the chatBuilder without the currentResponse
	// so that we can just rerender the markdown of the current response and append that to the builder and then return that string

	numPrompts, numResponses := c.numPrompts(), c.numResponses()
	if numPrompts == 0 && numResponses == 0 {
		return ""
	}

	// viewport width has changed. we must now re-render all prompts and responses so they wrap correctly
	if vpWidth != c.currentWrapWidth {
		c.renderedHistory.Reset()

		// Pre-calculate total size for both prompts and responses
		totalSize := 0
		for i := range len(c.history) {
			totalSize += len(c.history[i].prompt)*2 + len(c.history[i].response)
		}
		totalSize += c.CurrentResponse.Len()
		totalSize = int(float64(totalSize) * 1.5) // assuming markdown+ansi will add max 50% more bytes
		c.renderedHistory.Grow(totalSize)

		// Render chat
		c.numChatsRendered = c.renderChatHistory(0, vpWidth)
	} else {
		// when we have a new prompt or response, append to renderedHistory the latest rendered prompt/response
		if c.numChatsRendered < max(numPrompts, numResponses) {
			c.numChatsRendered = c.renderChatHistory(c.numChatsRendered, vpWidth)
		}
	}

	renderedHistory := c.renderedHistory.String()

	// Render current response being streamed
	if c.CurrentResponse.Len() > 0 {
		renderedHistory += c.renderCurrentResponse()
	}
	return renderedHistory
}

// renderChatHistory iterates through the chat history starting at the given index and writes to .renderedHistory text to display
// on screen. If the viewport width has changed since the last render, the prompt will be resized accordingly. It is assumed that the
// .Markdown renderer has already been resized before this function is called.
func (c *ChatModel) renderChatHistory(startingIndex, vpWidth int) int {
	textWidth := int(float64(vpWidth) * styles.PROMPT_WIDTH_PROPORTION)
	marginStyle := lipgloss.NewStyle().Width(vpWidth - textWidth)
	contentStyle := lipgloss.NewStyle().Inherit(c.styles.PromptText).Width(vpWidth)

	numChats := len(c.history)
	for i := startingIndex; i < numChats; i++ {
		prompt, response, error :=
			c.history[i].createFormattedPrompt(marginStyle, contentStyle, textWidth),
			c.history[i].response,
			c.history[i].error

		c.renderedHistory.WriteString(prompt)
		c.renderedHistory.WriteString(c.Markdown.Render(response))

		if len(error) > 0 {
			c.renderedHistory.WriteString(c.Markdown.Render(error))
		}
	}
	return numChats
}

func (c *ChatModel) renderCurrentResponse() string {
	if c.CurrentResponse.ResponseContent.Len() > 0 {
		return c.Markdown.Render(c.CurrentResponse.ResponseContent.String())
	}
	// TODO: don't print reasoning if model doesn't support (haiku) or user said no reasoning
	return c.Markdown.Render(c.CurrentResponse.ReasoningContent.String())

}

func (c *ChatModel) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	c.history = c.history[:0]
}
