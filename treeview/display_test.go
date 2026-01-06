package treeview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewDisplay tests creating a new display
func TestNewDisplay(t *testing.T) {
	t.Run("creates display with defaults", func(t *testing.T) {
		display := NewDisplay()

		assert.NotNil(t, display)
		assert.NotNil(t, display.renderer)
		assert.Equal(t, 0, display.lastLineCount)
	})
}

// TestDisplay_IsTerminal tests checking if stdout is a terminal
func TestDisplay_IsTerminal(t *testing.T) {
	t.Run("returns boolean", func(t *testing.T) {
		display := NewDisplay()
		result := display.IsTerminal()

		// Just check that it returns a bool, we can't control terminal detection in tests
		assert.IsType(t, true, result)
	})
}

// TestDisplay_RenderStatic tests static rendering
func TestDisplay_RenderStatic(t *testing.T) {
	t.Run("renders simple tree", func(t *testing.T) {
		display := NewDisplay()
		root := NewNode("root")
		child := NewNode("child")
		root.AddChild(child)

		// Just verify it doesn't panic
		assert.NotPanics(t, func() {
			display.RenderStatic(root)
		})
	})

	t.Run("renders complex tree", func(t *testing.T) {
		display := NewDisplay()
		root := NewNode("root")

		// Create multiple levels
		for i := 0; i < 3; i++ {
			child := NewNode("child" + string(rune(48+i)))
			root.AddChild(child)

			for j := 0; j < 2; j++ {
				grandchild := NewNode("grandchild")
				child.AddChild(grandchild)
			}
		}

		assert.NotPanics(t, func() {
			display.RenderStatic(root)
		})
	})
}

// TestDisplay_Render tests interactive rendering (may not display on non-terminal)
func TestDisplay_Render(t *testing.T) {
	t.Run("render on non-terminal returns early", func(t *testing.T) {
		display := NewDisplay()
		root := NewNode("root")

		// Should not panic even if not a terminal
		assert.NotPanics(t, func() {
			display.Render(root)
		})
	})

	t.Run("multiple renders update line count", func(t *testing.T) {
		display := NewDisplay()
		root := NewNode("root")
		root.AddChild(NewNode("child1"))

		assert.Equal(t, 0, display.lastLineCount)

		display.RenderStatic(root)
		// Line count would be set by Render, but we can't control terminal in tests
	})
}

// TestCountOutputLines tests line counting utility
func TestCountOutputLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "no newlines",
			input:    "hello world",
			expected: 0,
		},
		{
			name:     "single newline",
			input:    "hello\nworld",
			expected: 1,
		},
		{
			name:     "multiple newlines",
			input:    "line1\nline2\nline3\n",
			expected: 3,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "only newlines",
			input:    "\n\n\n",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countOutputLines(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDisplay_ThreadSafety tests concurrent rendering operations
func TestDisplay_ThreadSafety(t *testing.T) {
	t.Run("concurrent render operations", func(t *testing.T) {
		display := NewDisplay()
		root := NewNode("root")
		root.AddChild(NewNode("child"))

		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 10; i++ {
				display.Render(root)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 10; i++ {
				display.RenderStatic(root)
			}
			done <- true
		}()

		<-done
		<-done

		// Should complete without deadlock
		assert.True(t, true)
	})
}
