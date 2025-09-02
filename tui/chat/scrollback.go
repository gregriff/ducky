package tui

import (
	"fmt"
)

// scrollback.Buffer provides functionality that enables the TUI textarea to cycle through the prompt history
// by performing read-only operations on ChatModel.history
type Buffer struct {
	history    *[]ChatEntry
	currentIdx int // corresponds to the prompt currently visible in the prompt textarea
	userInput  string
}

// Len returns the length of the chat history, +1, since we need to take into account the prompt
// the user is currently typing in these calculations
func (b *Buffer) Len() int {
	return len(*b.history) + 1
}

// GetPrompt returns the prompt at the given index
func (b *Buffer) GetPrompt(idx int) string {
	historyLen := len(*b.history)
	if 0 > idx || idx >= historyLen {
		panic(fmt.Sprintf("GetPrompt err...\n%v#", b))
	}
	return (*b.history)[idx].prompt
}

func (b *Buffer) NextPrompt() string {
	// if we're not searching
	if b.currentIdx == -1 {
		return ""
	}

	// if the user is viewing the last prompt in the list, we need to show them what they typed before cycling
	if b.currentIdx == len(*b.history)-1 {
		// this means they're no longer cycling through the history, so denote that with -1
		b.currentIdx = -1
		return b.userInput
	}

	if b.currentIdx > len(*b.history)-1 {
		panic(fmt.Sprintf("This shouldn't happen...\n%v#", b))
	}

	prompt := b.GetPrompt(b.currentIdx + 1)
	b.currentIdx += 1
	return prompt
}

// TODO: use visibleText as param to know if to save edited text as userInput?
func (b *Buffer) PrevPrompt(visibleText string) string {
	if b.currentIdx == -1 {
		b.userInput = visibleText // save user input to restore later

		mostRecentPromptIdx := len(*b.history) - 1
		prompt := b.GetPrompt(mostRecentPromptIdx)
		b.currentIdx = mostRecentPromptIdx
		return prompt
	}

	// if user is viewing the first prompt in the history and are still trying to go up
	if b.currentIdx == 0 {
		return ""
	}

	if b.currentIdx < 0 {
		panic(fmt.Sprintf("This shouldn't happen...\n%v#", b))
	}

	prompt := b.GetPrompt(b.currentIdx - 1)
	b.currentIdx -= 1
	return prompt
}
