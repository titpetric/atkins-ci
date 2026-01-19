package treeview

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// Display manages in-place tree rendering with ANSI cursor control.
type Display struct {
	lastLineCount int
	mu            sync.Mutex
	isTerminal    bool
	renderer      *Renderer
	finalOnly     bool
}

// NewDisplay creates a new display manager.
func NewDisplay() *Display {
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	return &Display{
		lastLineCount: 0,
		isTerminal:    isTerminal,
		renderer:      NewRenderer(),
		finalOnly:     false,
	}
}

// NewDisplayWithFinal creates a new display manager with final-only mode.
func NewDisplayWithFinal(finalOnly bool) *Display {
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	return &Display{
		lastLineCount: 0,
		isTerminal:    isTerminal && !finalOnly,
		renderer:      NewRenderer(),
		finalOnly:     finalOnly,
	}
}

// IsTerminal returns whether stdout is a TTY.
func (d *Display) IsTerminal() bool {
	return d.isTerminal
}

// Render outputs the tree, updating in-place if previously rendered.
func (d *Display) Render(root *Node) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Only render if stdout is a TTY (interactive terminal)
	if !d.isTerminal {
		return
	}

	if d.lastLineCount > 0 {
		// Move cursor up, clear to end of display
		fmt.Printf("\033[%dA\033[J", d.lastLineCount)
	}

	output := d.renderer.Render(root)
	fmt.Print(output)

	d.lastLineCount = countOutputLines(output)
}

// RenderStatic displays a static tree view (for list).
func (d *Display) RenderStatic(root *Node) {
	d.mu.Lock()
	defer d.mu.Unlock()

	output := d.renderer.RenderStatic(root)
	fmt.Print(output)
}

// countOutputLines counts the number of newlines in output
func countOutputLines(output string) int {
	count := 0
	for _, ch := range output {
		if ch == '\n' {
			count++
		}
	}
	return count
}
