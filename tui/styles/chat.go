package tui

import "github.com/charmbracelet/lipgloss"

type ChatStylesStruct struct {
	PromptText,
	ReasoningText,
	ErrorText lipgloss.Style
}

var ChatStyles = ChatStylesStruct{
	PromptText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#32cd32")),

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
