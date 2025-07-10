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

func (c *ChatEntry) setPromptPadding(style lipgloss.Style, width int) {
	fullStyle := style.
		PaddingLeft(width - lipgloss.Width(c.rawPrompt) - styles.H_PADDING*2).
		PaddingTop(styles.PROMPT_V_PADDING).
		PaddingBottom(styles.PROMPT_V_PADDING)
	c.prompt = fullStyle.Render(c.rawPrompt)
}
