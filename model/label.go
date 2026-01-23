package model

// Label represents a display label for a step or command.
type Label struct {
	Text       string              // The display text (e.g., "docker compose up" or "run: goimports -w .")
	Type       string              // The type of operation: "task", "run", "cmd", "cmds"
	ShowPrefix bool                // Whether to display the type prefix (e.g., "run:")
	Status     string              // Optional status indicator (e.g., "●", "✓", "✗")
	Color      func(string) string // Optional color function to apply to the label
}

// NewLabel creates a new label with the given text and type.
// Use builder methods like WithPrefix(), WithStatus(), and WithColor() to customize.
func NewLabel(text, labelType string) *Label {
	return &Label{
		Text: text,
		Type: labelType,
	}
}

// WithPrefix sets whether to show the type prefix in output.
func (l *Label) WithPrefix(show bool) *Label {
	l.ShowPrefix = show
	return l
}

// WithStatus sets the status indicator to display.
func (l *Label) WithStatus(status string) *Label {
	l.Status = status
	return l
}

// WithColor sets a color function to apply to the label.
func (l *Label) WithColor(colorFn func(string) string) *Label {
	l.Color = colorFn
	return l
}

// String returns the formatted label as a clean string (no ANSI codes or status).
// Use this for node names and comparisons.
func (l *Label) String() string {
	text := l.Text
	if l.ShowPrefix {
		text = l.Type + ": " + text
	}
	return text
}

// ForDisplay returns the formatted label with color and status for rendering.
// This includes ANSI color codes and status indicators.
func (l *Label) ForDisplay() string {
	text := l.String()

	// Apply color if provided
	if l.Color != nil {
		text = l.Color(text)
	}

	// Append status if provided
	if l.Status != "" {
		text = text + " " + l.Status
	}

	return text
}
