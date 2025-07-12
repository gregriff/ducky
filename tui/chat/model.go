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

	builder  strings.Builder
	Markdown *MarkdownRenderer
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

func NewChatModel() *ChatModel {
	return &ChatModel{
		styles: &styles.ChatStyles,

		CurrentResponse: &CurrentResponse{},
		history:         make([]ChatEntry, 0, 10),
		Markdown:        NewMarkdownRenderer(),
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

// AddPrompt creates a new ChatEntry with prompt data given the current viewport width
func (c *ChatModel) AddPrompt(s string, vpWidth int) {
	textWidth := int(float64(vpWidth) * styles.PROMPT_WIDTH_PROPORTION)
	marginStyle := lipgloss.NewStyle().Width(vpWidth - textWidth)
	contentStyle := lipgloss.NewStyle().Inherit(c.styles.PromptText).Width(vpWidth)

	newEntry := &ChatEntry{rawPrompt: s}
	newEntry.setFormattedPrompt(marginStyle, contentStyle, textWidth)
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

// Render returns a string of the entire chat history in markdown and wrapped to a certain width
func (c *ChatModel) Render() string {
	// TODO: this runs every time a re-render happens so it is slower than the original approach
	// of keeping the chat history in a stringbuilder. We could still do that in this struct

	// TODO: only reset if width has changed (pass a width) and keep a copy of the chatBuilder without the currentResponse
	// so that we can just rerender the markdown of the current response and append that to the builder and then return that string

	defer c.builder.Reset()
	numPrompts, numResponses := c.numPrompts(), c.numResponses()
	if numPrompts == 0 && numResponses == 0 {
		return ""
	}

	// Pre-calculate total size for both prompts and responses
	totalSize := 0
	for i := range len(c.history) {
		totalSize += len(c.history[i].prompt) + len(c.history[i].reasoning) + len(c.history[i].response)
	}
	totalSize += c.CurrentResponse.Len()
	totalSize = int(float64(totalSize) * 1.4) // assuming markdown ansi will add max 40% more bytes
	c.builder.Grow(totalSize)

	// Render chat
	for i := range len(c.history) {
		prompt, response, error :=
			c.history[i].prompt,
			c.history[i].response,
			c.history[i].error

		// response at index N will correspond to prompt at index N
		c.builder.WriteString(prompt)

		// TODO: check for showReasoning
		// reasoningMarkdown := markdownRenderer.Render(h.ReasoningResponses[i])
		// reasoningFormatted := h.styles.ReasoningText.Render(reasoningMarkdown)
		// // reasoningFormatted := h.styles.ReasoningText.Render(h.ReasoningResponses[i])
		// h.chatBuilder.WriteString(reasoningFormatted)
		// h.chatBuilder.WriteString(markdownRenderer.Render("\n---\n"))

		c.builder.WriteString(c.Markdown.Render(response))

		if len(error) > 0 {
			c.builder.WriteString(c.Markdown.Render(error))
		}
	}

	// Render current response being streamed
	if c.CurrentResponse.Len() > 0 { // pretty much == "isStreaming" TODO: revise
		// TODO: don't print reasoning if model doesn't support (haiku) or user said no reasoning
		reasoningMarkdown := c.Markdown.Render(c.CurrentResponse.ReasoningContent.String())
		reasoningFormatted := c.styles.ReasoningText.Render(reasoningMarkdown) // TODO: cant render reasoning section if empty because of lipgloss formatting
		// reasoningFormatted := h.styles.ReasoningText.Render(currentResponse.reasoningContent.String())
		c.builder.WriteString(reasoningFormatted)

		if len(c.CurrentResponse.ResponseContent.String()) > 0 {
			c.builder.WriteString(c.Markdown.Render("\n---\n"))
		}

		c.builder.WriteString(c.Markdown.Render(c.CurrentResponse.ResponseContent.String()))
	}

	return c.builder.String()
}

func (c *ChatModel) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	c.history = c.history[:0]
}

// The below functions should go in another file

// ResizePrompts recreates h.Prompts for correct wrapping given the viewport width
func (c *ChatModel) ResizePrompts(vpWidth int) {
	// style := lipgloss.NewStyle().Inherit(c.styles.PromptText).Width(vpWidth)
	textWidth := int(float64(vpWidth) * styles.PROMPT_WIDTH_PROPORTION)
	marginStyle := lipgloss.NewStyle().Width(vpWidth - textWidth)
	contentStyle := lipgloss.NewStyle().Inherit(c.styles.PromptText).Width(vpWidth)

	// TODO: ensure c.history is aligned well
	for i := range len(c.history) {
		// c.history[i].setPromptPadding(style, vpWidth)
		c.history[i].setFormattedPrompt(marginStyle, contentStyle, textWidth)
	}
}
