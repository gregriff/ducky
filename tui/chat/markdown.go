package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

// MarkdownRenderer wraps glamour.TermRenderer and handles Markdown rendering
// as well as resizing the renderable area of the screen.
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
	wrapper  wrap.Wrap

	style string
}

// NewMarkdownRenderer creates the struct but Markdown cannot be rendered until .SetWidth is called
func NewMarkdownRenderer(glamourStyle string) *MarkdownRenderer {
	renderer, err := glamour.NewTermRenderer(
		// glamour.WithAutoStyle(), // this results in a hanging func call because of an ENOTTY
		glamour.WithStylePath(glamourStyle),
		glamour.WithEmoji(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Markdown renderer: %v\n", err)
		os.Exit(1)
	}
	// wrapper := wrap.NewWriter(80)
	// wrapper.TabWidth = 4
	return &MarkdownRenderer{
		style:    glamourStyle,
		renderer: renderer,
	}
}

// Render safely renders Markdown for a given width
func (md *MarkdownRenderer) Render(markdown []byte, width int) []byte {
	wrapper := wordwrap.NewWriter(width)
	// wrapper.TabWidth = 4

	rendered, err := md.renderer.RenderBytes(markdown)
	if err != nil {
		wrapper.Write(markdown) // Fallback to raw markdown
	} else {
		wrapper.Write(rendered)
	}
	wrapper.Close()
	return wrapper.Bytes()
}
