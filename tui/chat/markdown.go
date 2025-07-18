package tui

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer wraps glamour.TermRenderer and handles Markdown rendering
// as well as resizing the renderable area of the screen.
type MarkdownRenderer struct {
	mtx      sync.RWMutex
	renderer *glamour.TermRenderer

	style        string
	currentWidth int
}

// NewMarkdownRenderer creates the struct but Markdown cannot be rendered until .SetWidth is called
func NewMarkdownRenderer(glamourStyle string) *MarkdownRenderer {
	return &MarkdownRenderer{style: glamourStyle}
}

// Render safely renders Markdown
func (md *MarkdownRenderer) Render(markdown string) string {
	// this func could be called while a window resize is happening, so we lock
	md.mtx.RLock()
	defer md.mtx.RUnlock()

	renderer := md.renderer
	if renderer == nil {
		return markdown
	}

	output, err := renderer.Render(markdown)
	if err != nil {
		return markdown // Fallback to raw markdown
	}
	return output
}

// SetWidth immediately resizes the renderable area of the screen
func (md *MarkdownRenderer) SetWidth(width int) {
	md.mtx.Lock()
	defer md.mtx.Unlock()

	md.createRenderer(width)
}

// createRenderer actually does the work to enable Markdown to be rendered at a smaller or larger width.
// Re-instantiation is needed because glamour does not export the ansiOptions field
func (md *MarkdownRenderer) createRenderer(width int) {
	if md.currentWidth == width {
		return // No change needed
	}
	renderer, err := glamour.NewTermRenderer(
		// glamour.WithAutoStyle(), // this results in a hanging func call because of an ENOTTY
		glamour.WithStylePath(md.style),
		glamour.WithEmoji(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Markdown renderer: %v\n", err)
		os.Exit(1)
	}
	md.renderer = renderer
	md.currentWidth = width
}
