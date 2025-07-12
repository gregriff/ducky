package tui

import "github.com/charmbracelet/lipgloss"

type TUIStylesStruct struct {
	TitleBar,
	InputArea,
	PromptText,
	TextAreaCursor lipgloss.Style
}

// makeStyles declares formatting for text throughout the TUI
var TUIStyles = TUIStylesStruct{
	TitleBar: lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Faint(true).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, H_PADDING),

	InputArea: lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		Padding(0, H_PADDING),

	PromptText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#32cd32")),

	TextAreaCursor: lipgloss.NewStyle(),
}
