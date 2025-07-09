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
		// TODO: render history somewhere other than main viewport
		return t.showHistory(), true
	case ":clear":
		t.history.Clear()
		// TODO: render a message somewhere
		return "", true
	default:
		// TODO: render this somewhere else
		return "Unknown command", true
	}
}

func (t *TUI) showHistory() string {
	if len(t.history.RawPrompts) == 0 {
		return "No history"
	}

	var result strings.Builder
	for i, cmd := range t.history.RawPrompts {
		result.WriteString(fmt.Sprintf("%d: %s\n", i+1, cmd))
	}

	return strings.TrimSuffix(result.String(), "\n")
}
