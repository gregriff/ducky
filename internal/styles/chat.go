// Package styles contains lipgloss style constants for the text throughout the application, as well as spacing and sizing constants for the text and UI.
package styles

import "github.com/charmbracelet/lipgloss/v2"

// ChatStylesStruct defines styles for the text in the main viewport of the application (chat history).
type ChatStylesStruct struct {
	PromptText lipgloss.Style
}

var ChatStyles = ChatStylesStruct{
	PromptText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#32cd32")), // green

	// TODO: have reasoning use its own markdown renderer?
	// ReasoningText: lipgloss.NewStyle().
	// Foreground(lipgloss.Color("#a9a9a9")).
	// Foreground(lipgloss.Color("#32cd32")).
	// Faint(true).
	// PaddingLeft(H_PADDING),

	// ErrorText: lipgloss.NewStyle().
	// 	Foreground(lipgloss.Color("#32cd32")).
	// 	// Foreground(lipgloss.Color("#ff0000")).
	// 	PaddingBottom(PROMPT_V_PADDING).
	// 	PaddingTop(PROMPT_V_PADDING),
}
