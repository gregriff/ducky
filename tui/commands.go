/*
 * Commands the user can use in the TUI
 */
package tui

import (
	"fmt"
	"strings"
)

func (t *TUI) handleCommand(input string) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], ":") {
		return "", false
	}

	command := parts[0]
	switch command {
	case ":history":
		return t.showHistory(), true
	case ":clear":
		t.promptHistory = t.promptHistory[:0]
		t.chatHistory.Reset()
		t.addToChat("\nHistory cleared.\n\n---\n\n")
		return "", true
	default:
		return "Unknown command", true
	}
}

func (t *TUI) showHistory() string {
	if len(t.promptHistory) == 0 {
		return "No history"
	}

	var result strings.Builder
	for i, cmd := range t.promptHistory {
		result.WriteString(fmt.Sprintf("%d: %s\n", i+1, cmd))
	}

	return strings.TrimSuffix(result.String(), "\n")
}
