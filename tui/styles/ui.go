package tui

import "github.com/charmbracelet/lipgloss"

type TUIStylesStruct struct {
	TitleBar,
	PromptText,
	TextAreaCursor lipgloss.Style
}

// makeStyles declares formatting for text throughout the TUI
var TUIStyles = TUIStylesStruct{
	TitleBar: lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")). // cream
		Faint(true).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, H_PADDING),

	PromptText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCD4FF")), // light blue

	TextAreaCursor: lipgloss.NewStyle(),
}
