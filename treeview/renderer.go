package treeview

import (
	"fmt"
	"strings"
	"sync"

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

	// Render only the children, not the root again
	children := root.GetChildren()
	for i, child := range children {
		isLast := i == len(children)-1
		output += r.renderStaticNode(child, "", isLast)
	}

	return output
}

func statusBadge(node *Node) (string, string) {
	var badge string

	var (
		name         = node.Name
		label        = node.Name
		haveChildren = node.HasChildren()
		haveDeps     = len(node.Dependencies) > 0
	)

	switch node.Status {
	case StatusRunning:
		if node.HasChildren() {
			label = colors.BrightOrange(name)
		} else {
			label = colors.White(name)
		}
		if node.Spinner != "" {
			badge = node.Spinner
		} else {
			if node.HasChildren() {
				badge = colors.BrightOrange("●")
			}
		}
	case StatusPassed:
		badge = colors.BrightGreen("✓")
		label = colors.BrightWhite(name)
	case StatusFailed:
		badge = colors.BrightRed("✗")
		label = colors.BrightRed(name)
	case StatusSkipped:
		badge = colors.BrightYellow("⊘")
		label = colors.BrightYellow(name)
	case StatusConditional:
		badge = colors.Gray("●")
		label = colors.BrightYellow(name)
	default:
		if haveChildren || haveDeps {
			label = colors.BrightOrange(name)
			badge = colors.Green("●")
		} else {
			label = colors.White(name)
		}
	}
	// Only show "(deferred)" label if status is still pending/not started
	if node.Deferred && node.Status == StatusPending {
		label = label + " " + colors.Gray("(deferred)")
	}
	return badge, label
}

// renderStaticNode renders a static node without execution state (for list views)
func (r *Renderer) renderStaticNode(node *Node, prefix string, isLast bool) string {
	output := ""

	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	status, label := statusBadge(node)

	// Replace newlines in label to prevent breaking the tree display
	label = strings.ReplaceAll(label, "\n", " ")

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
