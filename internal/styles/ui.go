package styles

import "github.com/charmbracelet/lipgloss/v2"

// TUIStylesStruct defines style constants for the UI of the application.
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
