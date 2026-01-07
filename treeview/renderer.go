package treeview

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/titpetric/atkins-ci/colors"
)

// Renderer handles rendering of tree nodes to strings with proper formatting.
type Renderer struct {
	mu sync.Mutex
}

// NewRenderer creates a new tree renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render converts a node to a string representation.
func (r *Renderer) Render(root *Node) string {
	return r.RenderStatic(root)
}

// RenderStatic renders a static tree (for list views) without spinners.
func (r *Renderer) RenderStatic(root *Node) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	output := colors.BrightWhite(root.Name) + "\n"

	if root.Summarize {
		output += r.renderNodeSummary(root, "", true)
		return output
	}

	children := root.GetChildren()
	for i, child := range children {
		isLast := i == len(children)-1
		output += r.renderStaticNode(child, "", isLast)
	}

	return output
}

// renderNodeSummary will give a one-liner with status (pending, running, passed...)
func (r *Renderer) renderNodeSummary(node *Node, prefix string, isLast bool) string {
	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	var pending, running, passing, failed int
	for _, child := range node.GetChildren() {
		switch child.Status {
		case StatusRunning:
			running++
		case StatusFailed:
			failed++
		case StatusPending:
			pending++
		case StatusPassed:
			passing++
		}
	}

	total := pending + running + passing
	summary := colors.White(fmt.Sprintf("%d/%d", passing, total))
	if total == passing {
		summary = colors.Green(fmt.Sprintf("%d/%d", passing, total))
	}

	// If no summary items, just show the node name
	if len(summary) == 0 {
		label := node.Label()
		status := node.StatusColor()
		output := prefix + branch + label
		if status != "" {
			output += " " + status
		}
		return output + "\n"
	}

	return prefix + branch + node.Label() + " " + node.StatusColor() + " (" + colors.Gray(summary) + ")\n"
}

// renderStaticNode renders a static node without execution state (for list views)
func (r *Renderer) renderStaticNode(node *Node, prefix string, isLast bool) string {
	output := ""

	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	if node.Summarize {
		return r.renderNodeSummary(node, prefix, isLast)
	}

	label := node.Label()
	status := node.StatusColor()

	// Build the node label with dependencies and deferred info
	if len(node.Dependencies) > 0 {
		depItems := make([]string, len(node.Dependencies))
		for j, dep := range node.Dependencies {
			depItems[j] = colors.BrightOrange(dep)
		}
		depsStr := strings.Join(depItems, ", ")
		label = label + fmt.Sprintf(" (depends_on: %s)", depsStr)
	}
	// Note: deferred label is already handled in statusBadge() function above

	// Render this node
	output += prefix + branch + label
	if status != "" {
		output += " " + status
	}
	output += "\n"

	// Render output lines from command execution (with proper indentation)
	if len(node.Output) > 0 {
		// Determine continuation character for output indentation
		continuation := "│  "
		if isLast {
			continuation = "   "
		}

		// Calculate max width of output lines for border (in runes)
		maxWidth := 0
		for _, outputLine := range node.Output {
			width := utf8.RuneCountInString(outputLine)
			if width > maxWidth {
				maxWidth = width
			}
		}

		// Add top border if 2+ elements (account for spaces around content)
		if len(node.Output) >= 2 {
			topBorder := prefix + continuation + colors.Gray("┌"+strings.Repeat("─", maxWidth+2)+"┐") + "\n"
			output += topBorder
		}

		// Add each output line with left/right borders
		for _, outputLine := range node.Output {
			// Pad line to max width for consistent border
			currentWidth := utf8.RuneCountInString(outputLine)
			padding := strings.Repeat(" ", maxWidth-currentWidth)
			paddedLine := " " + outputLine + padding + " "
			if len(node.Output) >= 2 {
				output += prefix + continuation + colors.Gray("│") + colors.White(paddedLine) + colors.Gray("│") + "\n"
			} else {
				output += prefix + continuation + colors.White(outputLine) + "\n"
			}
		}

		// Add bottom border if 2+ elements (account for spaces around content)
		if len(node.Output) >= 2 {
			bottomBorder := prefix + continuation + colors.Gray("└"+strings.Repeat("─", maxWidth+2)+"┘") + "\n"
			output += bottomBorder
		}
	}

	// Render children
	children := node.GetChildren()
	if len(children) > 0 {
		// Determine continuation character
		continuation := "│  "
		if isLast {
			continuation = "   "
		}

		for j, child := range children {
			childIsLast := j == len(children)-1
			output += r.renderStaticNode(child, prefix+continuation, childIsLast)
		}
	}

	return output
}

// CountLines returns the number of lines the tree will render.
func CountLines(root *Node) int {
	count := 1 // root line
	children := root.GetChildren()
	for _, child := range children {
		count += countNodeLines(child)
	}
	return count
}

func countNodeLines(node *Node) int {
	count := 1 // this node
	children := node.GetChildren()
	for _, child := range children {
		count += countNodeLines(child)
	}
	return count
}

// sortChildrenByStatus reorders children so completed items appear first
// Order: Passed, Failed, Skipped, Running, Pending
func sortChildrenByStatus(children []*Node) []*Node {
	sorted := make([]*Node, len(children))
	copy(sorted, children)

	// Stable sort to maintain order within each status group
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Status != sorted[j+1].Status {
				if statusPriority(sorted[j].Status) < statusPriority(sorted[j+1].Status) {
					sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
				}
				continue
			}
		}
	}

	return sorted
}

// statusPriority returns a priority number for sorting (higher = comes first)
func statusPriority(status Status) int {
	switch status {
	case StatusPassed:
		return 4
	case StatusFailed:
		return 3
	case StatusSkipped:
		return 2
	case StatusRunning:
		return 2
	case StatusPending:
		return 1
	default:
		return 0
	}
}
