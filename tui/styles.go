package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	TitleBar,
	InputArea lipgloss.Style
}

func makeStyles(r *lipgloss.Renderer) (s styles) {
	titleBorder := lipgloss.RoundedBorder()
	titleBorder.Right = "├"
	s.TitleBar = r.NewStyle().
		BorderStyle(titleBorder).
		Padding(0, 1)

	s.InputArea = r.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		Padding(0, 1)
	return s
}
