package chat

import (
	"fmt"
	"os"

	"charm.land/glamour/v2"
)

// MarkdownRenderer wraps glamour.TermRenderer and handles Markdown rendering
// as well as resizing the renderable area of the screen.
type MarkdownRenderer struct {
	renderer     *glamour.TermRenderer
	CurrentWidth int

	style string
}

// NewMarkdownRenderer creates the struct but Markdown cannot be rendered until .SetWidth is called.
func NewMarkdownRenderer(glamourStyle string) *MarkdownRenderer {
	md := MarkdownRenderer{
		style:        glamourStyle,
		CurrentWidth: 80,
	}
	md.createNewRenderer()
	return &md
}

// createNewRenderer creates or re-creates the glamour.TermRenderer, using the curWidth and style fields.
func (md *MarkdownRenderer) createNewRenderer() {
	renderer, err := glamour.NewTermRenderer(
		// glamour.WithAutoStyle(), // this results in a hanging func call because of an ENOTTY
		glamour.WithStylePath(md.style),
		glamour.WithEmoji(),
		glamour.WithWordWrap(md.CurrentWidth),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Markdown renderer: %v\n", err)
		os.Exit(1)
	}
	md.renderer = renderer
}

// SetStyle will be used when the user wants to change the rendering style mid-session.
// The application should not allow the user to do this during rendering, because I don't want to add lock overhead.
func (md *MarkdownRenderer) SetStyle(newStyle string) {
	md.style = newStyle
}

// Render safely renders Markdown for a given width.
func (md *MarkdownRenderer) Render(markdown []byte, width int) []byte {
	// if the width has changed, recreate the renderer
	if width != md.CurrentWidth {
		md.CurrentWidth = width
		md.createNewRenderer()
	}

	rendered, err := md.renderer.RenderBytes(markdown)
	if err != nil {
		return markdown
	}
	return rendered
}
