package tui

import "github.com/charmbracelet/lipgloss/v2"

type TUIStylesStruct struct {
	TitleBar,
	PromptText,
	Spinner,
	TextAreaCursor lipgloss.Style
}

var (
	ColorPrimary   = lipgloss.Color("86")      // cream
	ColorSecondary = lipgloss.Color("#CCD4FF") // light blue
)

// makeStyles declares formatting for text throughout the TUI.
var TUIStyles = TUIStylesStruct{
	TitleBar: lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Faint(true).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, H_PADDING),

	PromptText: lipgloss.NewStyle().
		Foreground(ColorSecondary),

	Spinner: lipgloss.NewStyle().
		Foreground(ColorPrimary),

	TextAreaCursor: lipgloss.NewStyle(),
}
