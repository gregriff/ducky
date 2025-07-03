package repl

import "unicode"

func IsValidPromptInput(s string) bool {
	if len(s) != 1 {
		return false
	}
	return unicode.IsGraphic(rune(s[0]))
}
