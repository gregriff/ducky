/*
 * Commands the user can use in the TUI
 */
package tui

import (
	"strings"
)

func (t *TUI) handleCommand(input string) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], ":") {
		return "", false
	}

	command := parts[0]
	switch command {
	case ":clear":
		t.history.Clear()
		// TODO: render a message somewhere
		return "", true
	default:
		// TODO: render this somewhere else
		return "Unknown command", true
	}
}
