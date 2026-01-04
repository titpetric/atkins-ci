package treeview

import (
	"fmt"
	"strings"
	"sync"

	"github.com/titpetric/atkins-ci/colors"
)

// Renderer handles rendering of tree nodes to strings with proper formatting
type Renderer struct {
	mu sync.Mutex
}

// NewRenderer creates a new tree renderer
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render converts a node to a string representation
func (r *Renderer) Render(root *Node) string {
	return r.RenderStatic(root)
}

func (r *Renderer) Render_old(root *Node) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	output := colors.BrightGreen(root.Name) + "\n"
	children := sortChildrenByStatus(root.GetChildren())
	for i, child := range children {
		isLast := i == len(children)-1
		output += r.renderNode(child, "", isLast)
	}
	return output
}

// RenderStatic renders a static tree (for list views) without spinners
func (r *Renderer) RenderStatic(root *Node) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	output := colors.BrightGreen(root.Name) + "\n"

	//return output + r.renderNode(root, "", true)
	return output + r.renderStaticNode(root, "", true)
}

func statusBadge(node *Node) (string, string) {
	var badge string
	var nameColor string

	name, nameColor := node.Name, node.Name

	switch node.Status {
	case StatusRunning, StatusPending:
		if node.HasChildren() {
			nameColor = colors.BrightOrange(name)
		} else {
			nameColor = colors.White(name)
		}
		if node.Spinner != "" {
			badge = node.Spinner
		} else {
			badge = ""
		}
	case StatusPassed:
		badge = colors.BrightGreen("✓")
		nameColor = colors.BrightGreen(name)
	case StatusFailed:
		badge = colors.BrightRed("✗")
		nameColor = colors.BrightRed(name)
	case StatusSkipped:
		badge = colors.BrightYellow("⊘")
		nameColor = colors.BrightYellow(name)
	case StatusConditional:
		badge = colors.BrightYellow("~")
		nameColor = colors.BrightYellow(name)
	default:
	}
	return badge, nameColor
}

// renderNode renders a single node in the tree (used for execution views with spinners)
func (r *Renderer) renderNode(node *Node, prefix string, isLast bool) string {
	output := ""

	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	status, nameColor := statusBadge(node)

	isJob := len(node.GetChildren()) > 0

	// Build the node label with dependencies and deferred info
	nodeLabel := nameColor
	if isJob && len(node.Dependencies) > 0 {
		depItems := make([]string, len(node.Dependencies))
		for j, dep := range node.Dependencies {
			depItems[j] = colors.BrightOrange(dep)
		}
		depsStr := colors.BrightWhite(" (depends_on: ") + colors.White(strings.Join(depItems, ", ")) + colors.BrightWhite(")")
		nodeLabel = nodeLabel + depsStr
	}
	if node.Deferred {
		nodeLabel = nodeLabel + " " + colors.BrightCyan("(deferred)")
	}

	// Render this node
	output += prefix + branch + nodeLabel
	if status != "" {
		output += " " + status
	}
	output += "\n"

	// Render children (sorted by completion status)
	children := node.GetChildren()
	if len(children) > 0 {
		// Determine continuation character
		continuation := "│  "
		if isLast {
			continuation = "   "
		}

		// Sort children by status
		sortedChildren := sortChildrenByStatus(children)

		for j, child := range sortedChildren {
			childIsLast := j == len(sortedChildren)-1
			output += r.renderNode(child, prefix+continuation, childIsLast)
		}
	}

	return output
}

// renderStaticNode renders a static node without execution state (for list views)
func (r *Renderer) renderStaticNode(node *Node, prefix string, isLast bool) string {
	output := ""

	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	status, nameColor := statusBadge(node)

	// Build the node label with dependencies and deferred info
	nodeLabel := nameColor
	if node.HasChildren() && len(node.Dependencies) > 0 {
		depItems := make([]string, len(node.Dependencies))
		for j, dep := range node.Dependencies {
			depItems[j] = colors.BrightOrange(dep)
		}
		depsStr := strings.Join(depItems, ", ")
		nodeLabel = nodeLabel + fmt.Sprintf(" (depends_on: %s)", depsStr)
	}
	if node.Deferred {
		nodeLabel = nodeLabel + " " + colors.BrightCyan("(deferred)")
	}

	// Render this node
	output += prefix + branch + nodeLabel
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

// CountLines returns the number of lines the tree will render
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
