package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	TitleBar,
	InputArea,

	userText lipgloss.Style
}

const H_PADDING int = 1

func makeStyles(r *lipgloss.Renderer) (s Styles) {
	s.TitleBar = r.NewStyle().
		Foreground(lipgloss.Color("86")).
		Faint(true).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, H_PADDING)

	s.InputArea = r.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		Padding(0, H_PADDING)
	return s
}
