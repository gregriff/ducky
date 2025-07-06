package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
)

// RendererManager handles the creation and recreation of the Glamour Markdown Renderer.
// Since the viewport where the Markdown is rendered can change width when the user resizes the window,
// this needs to be able to be re-instantiated. And since rendering can be happening when resizing happens,
// we need Mutexes to prevent errors
type RendererManager struct {
	mu           sync.RWMutex
	currentWidth int
	renderer     *glamour.TermRenderer

	// Debouncing
	resizeTimer   *time.Timer
	pendingWidth  int
	debounceDelay time.Duration
}

func NewRendererManager() *RendererManager {
	return &RendererManager{
		currentWidth:  -1, // width on init will be zero, need to use -1 to prevent deadlock
		debounceDelay: 100 * time.Millisecond,
	}
}

func (mgr *RendererManager) createRenderer(width int) *glamour.TermRenderer {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithEmoji(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Markdown renderer: %v\n", err)
		os.Exit(1)
	}
	return renderer
}

// SetWidth handles resize events with debouncing
func (mgr *RendererManager) SetWidth(width int) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.pendingWidth = width

	// Cancel existing timer
	if mgr.resizeTimer != nil {
		mgr.resizeTimer.Stop()
	}

	// Set new timer
	mgr.resizeTimer = time.AfterFunc(mgr.debounceDelay, func() {
		mgr.applyWidth(mgr.pendingWidth)
	})
}

func (mgr *RendererManager) applyWidth(width int) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.currentWidth == width {
		return // No change needed
	}

	// Create new renderer
	// TODO: handle error here and return nothing to keep existing renderer?
	mgr.renderer = mgr.createRenderer(width)
	mgr.currentWidth = width
}

// Render safely renders markdown with current renderer
func (mgr *RendererManager) Render(markdown string) (string, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	renderer := mgr.renderer
	if renderer == nil {
		return markdown, nil // Fallback to raw markdown
	}

	return renderer.Render(markdown)
}

// ForceCreation immediately creates a new Renderer with the specified width
func (mgr *RendererManager) ForceCreation(width int) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.resizeTimer != nil {
		mgr.resizeTimer.Stop()
		mgr.resizeTimer = nil
	}

	mgr.applyWidth(width)
}
