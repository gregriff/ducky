package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer wraps glamour.TermRenderer and handles Markdown rendering
// as well as resizing the renderable area of the screen.
type MarkdownRenderer struct {
	mtx      sync.RWMutex
	renderer *glamour.TermRenderer

	style          string
	currentWidth   int
	pendingWidth   int
	resizeTimer    *time.Timer
	resizeDebounce time.Duration
}

func NewMarkdownRenderer(glamourStyle string) *MarkdownRenderer {
	return &MarkdownRenderer{
		style:          glamourStyle,
		currentWidth:   -1, // width on init will be zero, need to use -1 to prevent deadlock
		resizeDebounce: 50 * time.Millisecond,
	}
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

// SetWidth resizes the renderable area of the screen. Designed to handle frequent resize events by debouncing
func (md *MarkdownRenderer) SetWidth(width int) {
	md.mtx.Lock()
	defer md.mtx.Unlock()

	md.pendingWidth = width

	// Cancel existing timer
	if md.resizeTimer != nil {
		md.resizeTimer.Stop()
	}

	// Set new timer
	md.resizeTimer = time.AfterFunc(md.resizeDebounce, func() {
		md.applyWidth(md.pendingWidth)
	})
}

// SetWidthImmediate immediately resizes the renderable area of the screen
func (md *MarkdownRenderer) SetWidthImmediate(width int) {
	md.mtx.Lock()
	defer md.mtx.Unlock()

	if md.resizeTimer != nil {
		md.resizeTimer.Stop()
		md.resizeTimer = nil
	}

	md.createRenderer(width)
}

// applyWidth resizes the renderable area of the screen. Designed to be called by a delayed function
func (md *MarkdownRenderer) applyWidth(width int) {
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
