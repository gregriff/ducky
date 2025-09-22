package tui

import (
	"bytes"

	"github.com/charmbracelet/lipgloss/v2"
	styles "github.com/gregriff/ducky/internal/styles"
)

// ChatModel stores the state of the current chat with the LLM and formats prompts/responses
type ChatModel struct {
	history    []ChatEntry
	stream     *ResponseStream
	Scrollback *Traverser
	TotalCost  float64

	renderedHistory  bytes.Buffer // stores accumulated chat history rendered in markdown and color for a specific width
	Markdown         *MarkdownRenderer
	numChatsRendered int
}

// ResponseStream is like a buffer for the text sent from an LLM API. Once a response ends this data is moved into a ChatEntry
type ResponseStream struct {
	reasoning bytes.Buffer
	response  bytes.Buffer
	error     string
}

// Len returns the total byte count of the resoning and response parts of the current response
func (s *ResponseStream) Len() int {
	return s.reasoning.Len() + s.response.Len()
}

func NewChatModel(glamourStyle string) *ChatModel {
	model := ChatModel{
		stream:   &ResponseStream{},
		history:  make([]ChatEntry, 0, 10),
		Markdown: NewMarkdownRenderer(glamourStyle),
	}
	model.Scrollback = NewTraverser(&model.history)
	return &model
}

func (c *ChatModel) numPrompts() int {
	return len(c.history) // prompts determine the creation of new chat entries
}

func (c *ChatModel) numResponses() int {
	totalResponses := 0
	for i := range len(c.history) {
		if len(c.history[i].response) > 0 {
			totalResponses += 1
		}
	}
	return totalResponses
}

// AccumulateStream takes a chunk of streamed text from an LLM API, or an error message and records it to the response stream
// for later processing/storage
func (c *ChatModel) AccumulateStream(chunk string, isReasoning, isError bool) {
	if isError {
		c.stream.error = chunk
		return
	}

	if isReasoning {
		c.stream.reasoning.WriteString(chunk)
	} else {
		c.stream.response.WriteString(chunk)
	}
}

// AddPrompt creates a new ChatEntry with prompt data
func (c *ChatModel) AddPrompt(s string) {
	c.history = append(c.history, ChatEntry{prompt: s})
}

// AddResponse updates the latest ChatEntry with the data from ResponseStream. Must be called after AddPrompt
func (c *ChatModel) AddResponse() {
	stream := c.stream

	curEntry := &c.history[len(c.history)-1]
	curEntry.reasoning = stream.reasoning.String()

	curEntry.response = make([]byte, stream.response.Len())
	copy(curEntry.response, stream.response.Bytes())
	curEntry.error = stream.error

	stream.reasoning.Reset()
	stream.response.Reset()
	stream.error = ""
}

// Render returns a string of the entire chat history in markdown, wrapped to a certain width. If the vpWidth hasn't changed since the
// last call to this func, the pre-rendered chat history will be reused. If streaming, only the streamed response is returned, for UX reasons
func (c *ChatModel) Render(vpWidth int) string {
	numChatEntries := max(c.numPrompts(), c.numResponses())
	if numChatEntries == 0 {
		return ""
	}
	responseWidth := int(float64(vpWidth) * styles.WIDTH_PROPORTION_RESPONSE)

	// only render stream if streaming
	if c.stream.Len() > 0 {
		var renderedBytes []byte
		if c.stream.response.Len() > 0 {
			renderedBytes = c.Markdown.Render(c.stream.response.Bytes(), responseWidth)
		} else {
			// TODO: don't print reasoning if model doesn't support (haiku) or user said no reasoning
			renderedBytes = c.Markdown.Render(c.stream.reasoning.Bytes(), responseWidth)
		}
		return string(renderedBytes)
	}

	// else, render entire history
	// viewport width has changed. we must now re-render all prompts and responses so they wrap correctly
	if vpWidth != c.Markdown.CurrentWidth {
		c.renderedHistory.Reset()
		c.numChatsRendered = c.renderChatHistory(0, vpWidth, responseWidth)
	} else {
		// when we have a new prompt or response, append to renderedHistory the latest rendered prompt/response
		if c.numChatsRendered < numChatEntries {
			c.numChatsRendered = c.renderChatHistory(c.numChatsRendered, vpWidth, responseWidth)
		}
	}
	return c.renderedHistory.String()
}

// renderChatHistory iterates through the chat history starting at the given index and writes to .renderedHistory text to display
// on screen. If the viewport width has changed since the last render, the text will be resized accordingly by c.Markdown.Render
func (c *ChatModel) renderChatHistory(startingIndex, vpWidth, resWidth int) (count int) {
	maxPromptWidth := int(float64(vpWidth) * styles.WIDTH_PROPORTION_PROMPT)
	marginText := lipgloss.NewStyle().Width(vpWidth - maxPromptWidth).Render("")
	promptStyle := lipgloss.NewStyle().Inherit(styles.ChatStyles.PromptText).Width(maxPromptWidth)

	count = len(c.history)
	for i := startingIndex; i < count; i++ {
		prompt, response, error :=
			c.history[i].formattedPrompt(marginText, promptStyle, maxPromptWidth),
			c.history[i].response,
			c.history[i].error

		c.renderedHistory.WriteString(prompt)
		c.renderedHistory.WriteString("\n")
		c.renderedHistory.Write(c.Markdown.Render(response, resWidth))

		if len(error) > 0 {
			c.renderedHistory.Write(c.Markdown.Render([]byte(error), resWidth))
		}
	}
	return
}

func (c *ChatModel) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	c.history = make([]ChatEntry, 0, 10)
	c.numChatsRendered = 0
	c.renderedHistory.Reset()
}

func (c *ChatModel) HistoryLen() int {
	return len(c.history)
}
