package tui

import (
	"fmt"
	"strings"
)

type ChatHistory struct {
	Prompts   []string        // unformatted user prompts
	Content   strings.Builder // formatted prompts+responses
	TotalCost float64

	styles Styles
}

func (h *ChatHistory) addPrompt(s string) {
	h.Prompts = append(h.Prompts, s)
	styledPrompt := h.styles.PromptText.Render(fmt.Sprintf("%s\n\n", s))
	h.Content.WriteString(styledPrompt)
}

func (h *ChatHistory) addResponse(s string) {
	styledResponse := h.styles.ResponseText.Render(fmt.Sprintf("%s\n\n---\n\n", s))
	h.Content.WriteString(styledResponse)
}
