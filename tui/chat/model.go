package tui

import (
	"bytes"
	"log"

	"github.com/charmbracelet/lipgloss/v2"
	styles "github.com/gregriff/ducky/tui/styles"
)

// ChatModel stores the state of the current chat with the LLM and formats prompts/responses
type ChatModel struct {
	history   []ChatEntry
	stream    *ResponseStream
	TotalCost float64

	renderedHistory      bytes.Buffer // stores accumulated chat history rendered in markdown and color for a specific width
	Markdown             *MarkdownRenderer
	currentWrapWidth     int // # of term columns the stored prompts are word-wrapped to fit into
	numChatsRendered     int
	renderedLastResponse bool
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
	return &ChatModel{
		stream:   &ResponseStream{},
		history:  make([]ChatEntry, 0, 10),
		Markdown: NewMarkdownRenderer(glamourStyle),
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
	res := c.stream

	curEntry := &c.history[len(c.history)-1]
	curEntry.reasoning = res.reasoning.String()

	curEntry.response = make([]byte, res.response.Len())
	copy(curEntry.response, res.response.Bytes())
	curEntry.error = res.error

	res.reasoning.Reset()
	res.response.Reset()
	res.error = ""
}

// Render returns a string of the entire chat history in markdown, wrapped to a certain width. If the vpWidth hasn't changed since the
// last call to this func, the pre-rendered chat history will be reused and the ResponseStream will be appended to it
func (c *ChatModel) Render(vpWidth int) (content string) {
	numPrompts, numResponses := c.numPrompts(), c.numResponses()
	if numPrompts == 0 && numResponses == 0 {
		return ""
	}
	responseWidth := int(float64(vpWidth) * styles.WIDTH_PROPORTION_RESPONSE)

	// viewport width has changed. we must now re-render all prompts and responses so they wrap correctly
	if vpWidth != c.currentWrapWidth {
		c.renderedHistory.Reset()
		c.numChatsRendered = c.renderChatHistory(0, vpWidth, responseWidth, false)
		c.currentWrapWidth = vpWidth
	} else {
		// when we have a new prompt or response, append to renderedHistory the latest rendered prompt/response
		if c.numChatsRendered < max(numPrompts, numResponses) {
			c.numChatsRendered = c.renderChatHistory(c.numChatsRendered, vpWidth, responseWidth, false)
		} else {
			if !c.renderedLastResponse {
				c.numChatsRendered = c.renderChatHistory(c.numChatsRendered, vpWidth, responseWidth, true)
				c.renderedLastResponse = true
			}
		}
	}

	// Render current response being streamed
	if c.stream.Len() > 0 {
		c.renderedLastResponse = false

		// reduce copying by building onto renderedHistory buffer then truncating it after we get the final result
		baseLen := c.renderedHistory.Len()
		c.renderedHistory.Write(c.renderCurrentResponse(responseWidth))
		content = c.renderedHistory.String()
		c.renderedHistory.Truncate(baseLen)
	} else {
		content = c.renderedHistory.String()
	}
	return
}

// renderChatHistory iterates through the chat history starting at the given index and writes to .renderedHistory text to display
// on screen. If the viewport width has changed since the last render, the prompt will be resized accordingly. It is assumed that the
// .Markdown renderer has already been resized before this function is called.
func (c *ChatModel) renderChatHistory(startingIndex, vpWidth, resWidth int, renderLastResponse bool) (count int) {
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

	if renderLastResponse {
		lastResponseIdx := max(startingIndex-1, 0)
		lastResponseEntry := c.history[lastResponseIdx]
		c.renderedHistory.Write(c.Markdown.Render(lastResponseEntry.response, resWidth))
		log.Println("rendering last response, error should show")
		log.Println("cur res: ", string(lastResponseEntry.response))
		log.Println("cur err: ", lastResponseEntry.error)
		if len(lastResponseEntry.error) > 0 {
			log.Println("error detected")
			c.renderedHistory.Write(c.Markdown.Render([]byte(lastResponseEntry.error), resWidth))
		}
	}
	return
}

func (c *ChatModel) renderCurrentResponse(width int) []byte {
	if c.stream.response.Len() > 0 {
		return c.Markdown.Render(c.stream.response.Bytes(), width)
	}
	// TODO: don't print reasoning if model doesn't support (haiku) or user said no reasoning
	return c.Markdown.Render(c.stream.reasoning.Bytes(), width)

}

func (c *ChatModel) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	c.history = c.history[:0]
	c.numChatsRendered = 0
	// c.renderedLastResponse = false
	c.renderedHistory.Reset()
}

func (c *ChatModel) HistoryLen() int {
	return len(c.history)
}
