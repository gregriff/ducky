package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	TitleBar,
	InputArea,
	PromptText,
	ReasoningText,
	ErrorText lipgloss.Style
}

const H_PADDING int = 1
const PROMPT_V_PADDING = 1

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

	s.PromptText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#32cd32"))

	s.ReasoningText = lipgloss.NewStyle().
		// Foreground(lipgloss.Color("#a9a9a9")).
		Foreground(lipgloss.Color("#32cd32")).
		// Faint(true).
		PaddingLeft(H_PADDING)

	s.ErrorText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		PaddingBottom(PROMPT_V_PADDING).
		PaddingTop(PROMPT_V_PADDING)
	return s
}
