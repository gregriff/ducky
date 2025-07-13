package tui

import (
	"github.com/charmbracelet/lipgloss"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
)

type ChatEntry struct {
	// unformatted for storage (but still valid Markdown)
	prompt,
	reasoning,
	response,
	error string
}

// createFormattedPrompt overwrites the .prompt field, setting it to a string with a margin and padding.
// this func is used to format the rawPrompt with a margin and left padding to render it like an iMessage user text
func (c *ChatEntry) createFormattedPrompt(marginStyle, contentStyle lipgloss.Style, maxWidth int) string {
	fullStyle := contentStyle.
		PaddingLeft(maxWidth - lipgloss.Width(c.prompt) - styles.H_PADDING*2).
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	return lipgloss.JoinHorizontal(lipgloss.Top, marginStyle.Render(""), fullStyle.Render(c.prompt))
}
