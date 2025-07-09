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

// ChatHistory stores the state of the current chat with the LLM and formats prompts/responses
// TODO: use a proper ChatEntry struct array for the related string arrays
type ChatHistory struct {
	styles *styles.ChatStylesStruct

	CurrentResponse *CurrentResponse
	Prompts         []string // ansi formatted for rendering

	// unformatted for storage (but still valid Markdown) TODO: make these private
	Responses          []string // Markdown renderer handles ANSI formatting for these
	ReasoningResponses []string
	RawPrompts         []string
	Errors             []string

	chatBuilder strings.Builder

	TotalCost float64
}

func NewChatHistory() *ChatHistory {
	const cap = 10
	return &ChatHistory{
		styles: &styles.ChatStyles,

		CurrentResponse: &CurrentResponse{},
		Prompts:         make([]string, 0, cap),
		RawPrompts:      make([]string, 0, cap),
		Responses:       make([]string, 0, cap),
	}
}

// AddPrompt persists a formatted and unformatted prompt string to memory given the current viewport width
func (h *ChatHistory) AddPrompt(s string, width int) {
	h.RawPrompts = append(h.RawPrompts, s)
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)
	styledPrompt := applyPromptPadding(s, style, width)
	h.Prompts = append(h.Prompts, styledPrompt)
}

// AddResponse persists an unformatted response string to memory and updates related state
func (h *ChatHistory) AddResponse() {
	s := h.CurrentResponse

	h.ReasoningResponses = append(h.ReasoningResponses, s.ReasoningContent.String())
	h.Responses = append(h.Responses, s.ResponseContent.String())
	h.Errors = append(h.Errors, s.ErrorContent)
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
	if len(h.Prompts) == 0 {
		return ""
	}

	// Pre-calculate total size for both prompts and responses
	totalSize := 0
	minLen := min(len(h.Prompts), len(h.Responses))

	for i := range minLen {
		totalSize += len(h.Prompts[i]) + len(h.ReasoningResponses[i]) + len(h.Responses[i])
	}

	totalSize += h.CurrentResponse.Len()
	totalSize = int(float64(totalSize) * 1.4) // assuming markdown ansi will add max 40% more bytes
	h.chatBuilder.Grow(totalSize)

	for i := range minLen {
		// response at index N will correspond to prompt at index N
		h.chatBuilder.WriteString(h.Prompts[i])

		// TODO: check for showReasoning
		// reasoningMarkdown := markdownRenderer.Render(h.ReasoningResponses[i])
		// reasoningFormatted := h.styles.ReasoningText.Render(reasoningMarkdown)
		// // reasoningFormatted := h.styles.ReasoningText.Render(h.ReasoningResponses[i])
		// h.chatBuilder.WriteString(reasoningFormatted)
		// h.chatBuilder.WriteString(markdownRenderer.Render("\n---\n"))

		h.chatBuilder.WriteString(markdownRenderer.Render(h.Responses[i]))

		if len(h.Errors[i]) > 0 {
			errorFormatted := h.styles.ErrorText.Render(h.Errors[i])
			h.chatBuilder.WriteString(errorFormatted)
		}
	}

	// if we just sent a prompt
	if minLen < len(h.Prompts) {
		h.chatBuilder.WriteString(h.Prompts[minLen])

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
	h.RawPrompts = h.RawPrompts[:0]
	h.Prompts = h.Prompts[:0]
	h.ReasoningResponses = h.ReasoningResponses[:0]
	h.Responses = h.Responses[:0]
}

// The below functions should go in another file

// ResizePrompts recreates h.Prompts for correct wrapping given a width
func (h *ChatHistory) ResizePrompts(width int) {
	style := lipgloss.NewStyle().Inherit(h.styles.PromptText).Width(width)

	for i, prompt := range h.RawPrompts {
		h.Prompts[i] = applyPromptPadding(prompt, style, width)
	}
}

func applyPromptPadding(prompt string, style lipgloss.Style, width int) string {
	fullStyle := style.
		PaddingLeft(width - lipgloss.Width(prompt) - styles.H_PADDING*2).
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	return fullStyle.Render(prompt)
}
