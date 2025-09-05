package tui

import (
	"fmt"
)

// scrollback.Traverser provides functionality that enables the TUI textarea to cycle through the prompt history
// by performing read-only operations on ChatModel.history. It's a bi-directional iterator over the prompt history
// that keeps track of modifications made to any prompt in the history, until the user submits the prompt.
// It attemps to emulate Python's REPL history traversal
type Traverser struct {
	history       *[]ChatEntry
	currentIdx    int            // corresponds to the prompt currently visible in the prompt textarea
	userInput     string         // what the user typed in the textarea before traversing the history
	editedPrompts map[int]string // stores idx:prompt mappings for prompts that the user modifies when traversing history
}

func NewTraverser(historyPtr *[]ChatEntry) *Traverser {
	return &Traverser{
		history:       historyPtr,
		currentIdx:    -1,
		editedPrompts: make(map[int]string),
	}
}

// getPrompt returns the prompt at the given index
func (t *Traverser) getPrompt(idx int) string {
	historyLen := len(*t.history)
	if 0 > idx || idx >= historyLen {
		panic(fmt.Sprintf("GetPrompt err...\n%#v\nHistoryLen:%d", t, len(*t.history)))
	}
	return (*t.history)[idx].prompt
}

// NextPrompt should not be called if the prompt history is empty
func (t *Traverser) NextPrompt(visibleText string) (prompt string, found bool) {
	// log.Printf("\nNEXT CALLED: %d\n", t.currentIdx)
	historyLen := len(*t.history)
	if historyLen == 0 {
		return "", false
	}

	// if we're not searching
	if t.currentIdx == -1 {
		return "", false
	}

	if t.currentIdx > historyLen-1 {
		panic(fmt.Sprintf("This shouldn't happen...\n%#v\nHistoryLen:%d", t, len(*t.history)))
	}

	if visibleText != t.getPrompt(t.currentIdx) {
		t.editedPrompts[t.currentIdx] = visibleText
	}

	// if the user is viewing the last prompt in the list, we need to show them what they typed before cycling
	if t.currentIdx == historyLen-1 {
		// this means they're no longer cycling through the history, so denote that with -1
		t.currentIdx = -1
		// log.Printf("\n\nNEXT CALLED AND USER SHOULD SEE THEIR TAIL:\nuserInput: %s", t.userInput)
		return t.userInput, true
	}

	nextPrompt := t.getPrompt(t.currentIdx + 1)
	t.currentIdx += 1

	if modifiedPrompt, exists := t.editedPrompts[t.currentIdx]; exists {
		return modifiedPrompt, true
	}

	return nextPrompt, true
}

// PrevPrompt should not be called if the prompt history is empty
func (t *Traverser) PrevPrompt(visibleText string) (prompt string, found bool) {
	// log.Printf("\nPREV CALLED: %d\n", t.currentIdx)
	historyLen := len(*t.history)
	if historyLen == 0 {
		return "", false
	}

	if t.currentIdx == -1 {
		t.userInput = visibleText // save user input to restore later
		mostRecentPromptIdx := historyLen - 1
		// log.Printf("\n\nUSER GOING BACK FROM TAIL. \nmostRecentPromptIdx: %d\nvisibleText: %s", mostRecentPromptIdx, visibleText)

		var prompt string
		if editedPrompt, exists := t.editedPrompts[mostRecentPromptIdx]; exists {
			prompt = editedPrompt
		} else {
			prompt = t.getPrompt(mostRecentPromptIdx)
		}
		t.currentIdx = mostRecentPromptIdx
		return prompt, true
	}

	// if user is viewing the first prompt in the history and are still trying to go up
	if t.currentIdx == 0 {
		return "", false
	}

	if t.currentIdx < 0 {
		panic(fmt.Sprintf("This shouldn't happen...\n%#v\nHistoryLen:%d", t, len(*t.history)))
	}

	// if user has modified the prompt on their screen since they traversed to it, then save their
	// modifications for the rest of their traversal
	if visibleText != t.getPrompt(t.currentIdx) {
		t.editedPrompts[t.currentIdx] = visibleText
	}

	prevPrompt := t.getPrompt(t.currentIdx - 1)
	t.currentIdx -= 1

	if modifiedPrompt, exists := t.editedPrompts[t.currentIdx]; exists {
		return modifiedPrompt, true
	}

	return prevPrompt, true
}

// Reset should be called whenever the user submits a prompt to the model
func (t *Traverser) Reset() {
	t.currentIdx = -1
	t.userInput = ""
	clear(t.editedPrompts)
}
