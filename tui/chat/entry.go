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

// createFormattedPrompt creates a prompt string formatted with margin and padding.
// It tries to emulate iMessage, right-justifying the prompt and adding a left margin
func (c *ChatEntry) createFormattedPrompt(marginStyle, promptStyle lipgloss.Style, maxWidth int) string {
	// var textWidth int
	// // if the prompt is one line and < maxWidth we want it justified right
	// if unformattedWidth := lipgloss.Width(c.prompt); unformattedWidth < maxWidth {
	// 	textWidth = unformattedWidth
	// } else {
	// 	// the prompt needs to be wrapped across multiple lines to fit in the maxWidth,
	// 	// so lets wrap it and use the width of the longest line
	// 	textWidth = lipgloss.Width(contentStyle.Render(c.prompt))
	// }
	fullPromptStyle := promptStyle.
		PaddingLeft(maxWidth - lipgloss.Width(c.prompt) - styles.H_PADDING*2). // replace lg.W with textWidth if bugged
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	return lipgloss.JoinHorizontal(lipgloss.Top, marginStyle.Render(""), fullPromptStyle.Render(c.prompt))
}
