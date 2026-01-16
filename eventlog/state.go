package eventlog

import (
	"github.com/titpetric/atkins/treeview"
)

// statusToResult converts a treeview.Status to a Result.
func statusToResult(status treeview.Status) Result {
	switch status {
	case treeview.StatusPassed:
		return ResultPass
	case treeview.StatusFailed:
		return ResultFail
	case treeview.StatusSkipped:
		return ResultSkipped
	default:
		return ""
	}
}

// NodeToStateNode converts a treeview.Node to a StateNode for serialization.
func NodeToStateNode(node *treeview.Node) *StateNode {
	if node == nil {
		return nil
	}

	state := &StateNode{
		Name:      node.Name,
		ID:        node.ID,
		Status:    node.Status.Label(),
		Result:    statusToResult(node.Status),
		If:        node.If,
		CreatedAt: node.CreatedAt,
		UpdatedAt: node.UpdatedAt,
		Start:     node.StartOffset,
		Duration:  node.Duration,
	}

	// Convert children
	if len(node.Children) > 0 {
		state.Children = make([]*StateNode, 0, len(node.Children))
		for _, child := range node.Children {
			if childState := NodeToStateNode(child); childState != nil {
				state.Children = append(state.Children, childState)
			}
		}
	}

	return state
}

// TreeNodeToStateNode converts a treeview.TreeNode to a StateNode.
func TreeNodeToStateNode(node *treeview.TreeNode) *StateNode {
	if node == nil {
		return nil
	}
	return NodeToStateNode(node.Node)
}

// CountSteps counts steps by result in a StateNode tree.
func CountSteps(node *StateNode) (total, passed, failed, skipped int) {
	if node == nil {
		return
	}

	// Count this node if it's a leaf (no children = actual step)
	if len(node.Children) == 0 {
		total++
		switch node.Result {
		case ResultPass:
			passed++
		case ResultFail:
			failed++
		case ResultSkipped:
			skipped++
		}
	}

	// Recurse into children
	for _, child := range node.Children {
		t, p, f, s := CountSteps(child)
		total += t
		passed += p
		failed += f
		skipped += s
	}

	return
}

// CalculateDuration calculates the total duration from node timing.
func CalculateDuration(node *StateNode) float64 {
	if node == nil {
		return 0
	}
	return node.Duration
}
