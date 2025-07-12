package tui

import (
	"github.com/charmbracelet/lipgloss"
	styles "github.com/gregriff/gpt-cli-go/tui/styles"
)

type ChatEntry struct {
	prompt, // ansi formatted for rendering

	// unformatted for storage (but still valid Markdown)
	rawPrompt,
	reasoning,
	response,
	error string
}

// setFormattedPrompt overwrites the .prompt field, setting it to a string with a margin and padding.
// this func is used to format the rawPrompt with a margin and left padding to render it like an iMessage user text
func (c *ChatEntry) setFormattedPrompt(marginStyle, contentStyle lipgloss.Style, maxWidth int) {
	fullStyle := contentStyle.
		PaddingLeft(maxWidth - lipgloss.Width(c.rawPrompt) - styles.H_PADDING*2).
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	c.prompt = lipgloss.JoinHorizontal(lipgloss.Top, marginStyle.Render(""), fullStyle.Render(c.rawPrompt))
}
