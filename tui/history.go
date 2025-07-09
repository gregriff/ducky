package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ChatHistory stores the state of the current chat with the LLM and formats prompts/responses
type ChatHistory struct {
	Prompts []string // ansi formatted for rendering

	// unformatted for storage (but still valid Markdown)
	Responses  []string // Markdown renderer handles ANSI formatting for these
	rawPrompts []string

	chatBuilder strings.Builder

	TotalCost float64
	styles    Styles
}

func NewChatHistory() *ChatHistory {
	const cap = 10
	return &ChatHistory{
		Prompts:    make([]string, 0, cap),
		rawPrompts: make([]string, 0, cap),
		Responses:  make([]string, 0, cap),
		styles:     makeStyles(),
	}
}

// AddPrompt persists a formatted and unformatted prompt string to memory
func (h *ChatHistory) AddPrompt(s string, width int) {
	h.rawPrompts = append(h.rawPrompts, s)
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)
	styledPrompt := h.changePromptWrapping(s, style, width)
	h.Prompts = append(h.Prompts, styledPrompt)
}

// AddResponse persists a formatted and unformatted response string to memory
func (h *ChatHistory) AddResponse(s string) {
	s = fmt.Sprintf("%s\n\n---\n\n", s)
	h.Responses = append(h.Responses, s)
}

// BuildChatString builds and returns a string of the entire chat history for rendering in the viewport
func (h *ChatHistory) BuildChatString(markdownRenderer *MarkdownRenderer, currentResponse *string) string {
	// TODO: this runs every time a re-render happens so it is slower than the original approach
	// of keeping the chat history in a stringbuilder. We could still do that in this struct

	defer h.chatBuilder.Reset()
	if len(h.Prompts) == 0 {
		return ""
	}

	// Pre-calculate total size for both prompts and responses
	totalSize := 0
	minLen := min(len(h.Prompts), len(h.Responses))

	for i := range minLen {
		totalSize += len(h.Prompts[i]) + len(h.Responses[i])
	}

	if currentResponse != nil {
		totalSize += len(*currentResponse)
	}
	totalSize = int(float64(totalSize) * 1.4) // assuming markdown ansi will add max 40% more bytes

	h.chatBuilder.Grow(totalSize)

	for i := range minLen {
		// response at index N will correspond to prompt at index N
		h.chatBuilder.WriteString(h.Prompts[i])
		h.chatBuilder.WriteString(markdownRenderer.Render(h.Responses[i]))
	}

	// if we just sent a prompt
	if minLen < len(h.Prompts) {
		h.chatBuilder.WriteString(h.Prompts[minLen])

		if currentResponse != nil {
			h.chatBuilder.WriteString(markdownRenderer.Render(*currentResponse))
		}
	}

	return h.chatBuilder.String()
}

func (h *ChatHistory) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	h.rawPrompts = h.rawPrompts[:0]
	h.Prompts = h.Prompts[:0]
	h.Responses = h.Responses[:0]
}

// ResizePrompts recreates h.Prompts for correct wrapping given a width. should probably go in another file
func (h *ChatHistory) ResizePrompts(width int) {
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)

	for i, prompt := range h.rawPrompts {
		h.Prompts[i] = h.changePromptWrapping(prompt, style, width)
	}
}

func (h *ChatHistory) changePromptWrapping(prompt string, style lipgloss.Style, width int) string {
	fullStyle := style.PaddingLeft(width - lipgloss.Width(prompt) - H_PADDING*10)
	return fullStyle.Render(prompt)
}
