package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
)

type CurrentResponse struct {
	ReasoningContent strings.Builder
	ResponseContent  strings.Builder
	ErrorContent     string
}

// isEmpty returns true if there is any text content or an error in the current response
func (res *CurrentResponse) isEmpty() bool {
	if res.Len() > 0 || len(res.ErrorContent) > 0 {
		return false
	}
	return true
}

// Len returns the total byte count of the resoning and response parts of the current response
func (res *CurrentResponse) Len() int {
	return res.ReasoningContent.Len() + res.ResponseContent.Len()
}

type ChatEntry struct {
	prompt, // ansi formatted for rendering

	// unformatted for storage (but still valid Markdown)
	rawPrompt,
	reasoningResponse,
	response,
	error string
}

// ChatHistory stores the state of the current chat with the LLM and formats prompts/responses
// TODO: use a proper ChatEntry struct array for the related string arrays
type ChatHistory struct {
	styles *styles.ChatStylesStruct

	CurrentResponse *CurrentResponse
	Content         []ChatEntry

	// Prompts         []string // ansi formatted for rendering

	// unformatted for storage (but still valid Markdown) TODO: make these private
	// Responses          []string // Markdown renderer handles ANSI formatting for these
	// ReasoningResponses []string
	// RawPrompts         []string
	// Errors             []string

	chatBuilder strings.Builder

	TotalCost float64
}

func NewChatHistory() *ChatHistory {
	return &ChatHistory{
		styles: &styles.ChatStyles,

		CurrentResponse: &CurrentResponse{},
		Content:         make([]ChatEntry, 0, 10),
	}
}

func (h *ChatHistory) numPrompts() int {
	return len(h.Content) // prompts determine the creation of new chat entries
}

func (h *ChatHistory) numResponses() int {
	totalResponses := 0
	for _, entry := range h.Content {
		if len(entry.response) > 0 { // TODO: should this take into account reasoningResponses?
			totalResponses += 1
		}
	}
	return totalResponses
}

// AddPrompt creates a new ChatEntry with prompt data given the current viewport width
func (h *ChatHistory) AddPrompt(s string, width int) {
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)
	styledPrompt := applyPromptPadding(s, style, width)
	h.Content = append(h.Content, ChatEntry{rawPrompt: s, prompt: styledPrompt})
}

// AddResponse updates the latest ChatEntry with the data from CurrentResponse. Must be called after AddPrompt
func (h *ChatHistory) AddResponse() {
	s := h.CurrentResponse

	curChatEntry := &h.Content[len(h.Content)-1]
	curChatEntry.reasoningResponse = s.ReasoningContent.String()
	curChatEntry.response = s.ResponseContent.String()
	curChatEntry.error = s.ErrorContent
	s.ReasoningContent.Reset()
	s.ResponseContent.Reset()
	s.ErrorContent = ""
}

// BuildChatString builds and returns a string of the entire chat history for rendering in the viewport
func (h *ChatHistory) BuildChatString(markdownRenderer *MarkdownRenderer) string {
	// TODO: this runs every time a re-render happens so it is slower than the original approach
	// of keeping the chat history in a stringbuilder. We could still do that in this struct

	// TODO: only reset if width has changed (pass a width) and keep a copy of the chatBuilder without the currentResponse
	// so that we can just rerender the markdown of the current response and append that to the builder and then return that string

	defer h.chatBuilder.Reset()
	numPrompts := h.numPrompts()
	if numPrompts == 0 {
		return ""
	}

	// Pre-calculate total size for both prompts and responses
	totalSize := 0
	minLen := min(numPrompts, h.numResponses())

	for i := range minLen {
		totalSize += len(h.Content[i].prompt) + len(h.Content[i].reasoningResponse) + len(h.Content[i].response)
	}

	totalSize += h.CurrentResponse.Len()
	totalSize = int(float64(totalSize) * 1.4) // assuming markdown ansi will add max 40% more bytes
	h.chatBuilder.Grow(totalSize)

	for i := range minLen {
		// response at index N will correspond to prompt at index N
		h.chatBuilder.WriteString(h.Content[i].prompt)

		// TODO: check for showReasoning
		// reasoningMarkdown := markdownRenderer.Render(h.ReasoningResponses[i])
		// reasoningFormatted := h.styles.ReasoningText.Render(reasoningMarkdown)
		// // reasoningFormatted := h.styles.ReasoningText.Render(h.ReasoningResponses[i])
		// h.chatBuilder.WriteString(reasoningFormatted)
		// h.chatBuilder.WriteString(markdownRenderer.Render("\n---\n"))

		h.chatBuilder.WriteString(markdownRenderer.Render(h.Content[i].response))

		if len(h.Content[i].error) > 0 {
			errorFormatted := h.styles.ErrorText.Render(h.Content[i].error)
			h.chatBuilder.WriteString(errorFormatted)
		}
	}

	// if we just sent a prompt
	if minLen < numPrompts {
		h.chatBuilder.WriteString(h.Content[minLen].prompt)

		// TODO: cant render reasoning or error sections if empty because of lipgloss formatting
		if !h.CurrentResponse.isEmpty() {
			reasoningMarkdown := markdownRenderer.Render(h.CurrentResponse.ReasoningContent.String())
			reasoningFormatted := h.styles.ReasoningText.Render(reasoningMarkdown)
			// reasoningFormatted := h.styles.ReasoningText.Render(currentResponse.reasoningContent.String())
			h.chatBuilder.WriteString(reasoningFormatted)

			if len(h.CurrentResponse.ResponseContent.String()) > 0 {
				h.chatBuilder.WriteString(markdownRenderer.Render("\n---\n"))
			}

			h.chatBuilder.WriteString(markdownRenderer.Render(h.CurrentResponse.ResponseContent.String()))

			if len(h.CurrentResponse.ErrorContent) > 0 {
				errorFormatted := h.styles.ErrorText.Render(h.CurrentResponse.ErrorContent)
				h.chatBuilder.WriteString(errorFormatted)
			}
		}
	}
	return h.chatBuilder.String()
}

func (h *ChatHistory) Clear() {
	// TODO: save unsaved history in temporary sqlite DB or in-memory for accidental clears
	h.Content = h.Content[:0]
}

// The below functions should go in another file

// ResizePrompts recreates h.Prompts for correct wrapping given a width
func (h *ChatHistory) ResizePrompts(width int) {
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)

	for _, entry := range h.Content {
		entry.prompt = applyPromptPadding(entry.rawPrompt, style, width)
	}
}

func applyPromptPadding(prompt string, style lipgloss.Style, width int) string {
	fullStyle := style.
		PaddingLeft(width - lipgloss.Width(prompt) - styles.H_PADDING*2).
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	return fullStyle.Render(prompt)
}
