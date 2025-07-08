package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	TitleBar,
	InputArea,
	PromptText,
	ResponseText,

	userText lipgloss.Style
}

const H_PADDING int = 1

// makeStyles declares formatting for text throughout the TUI
func makeStyles() (s Styles) {
	s.TitleBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Faint(true).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, H_PADDING)

	s.InputArea = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		Padding(0, H_PADDING)

	// TODO: align right wont work. need to find way to place prompt text on the right, maybe whitespace, maybe bubbles
	s.PromptText = lipgloss.NewStyle().
		// BorderStyle(lipgloss.NormalBorder())
		Foreground(lipgloss.Color("#32cd32")).
		AlignHorizontal(lipgloss.Right)
	s.ResponseText = lipgloss.NewStyle().
		AlignHorizontal(lipgloss.Left)
	return s
}
