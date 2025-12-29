// Package chat includes data structures that store the state of the chat history and functions that alter this history and render it into colored text
package chat

import (
	"charm.land/lipgloss/v2"
	styles "github.com/gregriff/ducky/internal/styles"
)

// Entry contains unformatted chat text for storage (but still valid Markdown).
type Entry struct {
	prompt,
	reasoning,
	error string

	response []byte
}

// formattedPrompt creates a prompt string formatted with margin and padding.
// It tries to emulate iMessage by adding a left margin.
func (c *Entry) formattedPrompt(marginText string, promptStyle lipgloss.Style, maxWidth int) string {
	fullPromptStyle := promptStyle.
		PaddingLeft(maxWidth - lipgloss.Width(c.prompt) - styles.H_PADDING*2). // replace lg.W with textWidth if bugged
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	return lipgloss.JoinHorizontal(lipgloss.Top, marginText, fullPromptStyle.Render(c.prompt))
}
