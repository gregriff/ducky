/*
 * Commands the user can use in the TUI
 */
package tui

import (
	"fmt"
	"strings"
)

func (r *TUI) handleSet(parts []string) string {
	if len(parts) < 3 {
		return "Usage: set <variable> <value>"
	}

	variable := parts[1]
	value := strings.Join(parts[2:], " ")
	r.vars[variable] = value

	return fmt.Sprintf("Set %s = %s", variable, value)
}

func (r *TUI) handleGet(parts []string) string {
	if len(parts) != 2 {
		return "Usage: get <variable>"
	}

	variable := parts[1]
	if value, exists := r.vars[variable]; exists {
		return value
	}

	return fmt.Sprintf("Variable '%s' not found", variable)
}

func (r *TUI) showHistory() string {
	if len(r.history) == 0 {
		return "No history"
	}

	var result strings.Builder
	for i, cmd := range r.history {
		result.WriteString(fmt.Sprintf("%d: %s\n", i+1, cmd))
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (r *TUI) showVars() string {
	if len(r.vars) == 0 {
		return "No variables set"
	}

	var result strings.Builder
	for key, value := range r.vars {
		result.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	return strings.TrimSuffix(result.String(), "\n")
}
