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
	case ":set":
		return t.handleSet(parts), true
	case ":get":
		return t.handleGet(parts), true
	case ":history":
		return t.showHistory(), true
	case ":clear":
		t.history = t.history[:0]
		t.chatHistory.Reset()
		t.addToChat("\nHistory cleared.\n\n---\n\n")
		return "", true
	case ":vars":
		return t.showVars(), true
	default:
		return "Unknown command", true
	}
}

func (t *TUI) handleSet(parts []string) string {
	if len(parts) < 3 {
		return "Usage: set <variable> <value>"
	}

	variable := parts[1]
	value := strings.Join(parts[2:], " ")
	t.vars[variable] = value

	return fmt.Sprintf("Set %s = %s", variable, value)
}

func (t *TUI) handleGet(parts []string) string {
	if len(parts) != 2 {
		return "Usage: get <variable>"
	}

	variable := parts[1]
	if value, exists := t.vars[variable]; exists {
		return value
	}

	return fmt.Sprintf("Variable '%s' not found", variable)
}

func (t *TUI) showHistory() string {
	if len(t.history) == 0 {
		return "No history"
	}

	var result strings.Builder
	for i, cmd := range t.history {
		result.WriteString(fmt.Sprintf("%d: %s\n", i+1, cmd))
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (t *TUI) showVars() string {
	if len(t.vars) == 0 {
		return "No variables set"
	}

	var result strings.Builder
	for key, value := range t.vars {
		result.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	return strings.TrimSuffix(result.String(), "\n")
}
