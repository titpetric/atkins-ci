package treeview

import (
	"sync"

	"github.com/titpetric/atkins/model"
)

// TreeNode represents a node in the execution tree (backward compatibility).
type TreeNode struct {
	*Node
	mu sync.Mutex
}

// NewTreeNode creates a new tree node.
func NewTreeNode(name string) *TreeNode {
	return &TreeNode{
		Node: NewNode(name),
	}
}

// ExecutionTree holds the entire execution tree.
type ExecutionTree struct {
	Root *TreeNode
	mu   sync.Mutex
}

// NewExecutionTree creates a new execution tree with a root node.
func NewExecutionTree(pipelineName string) *ExecutionTree {
	return &ExecutionTree{
		Root: &TreeNode{
			Node: &Node{
				Name:     pipelineName,
				Status:   StatusRunning,
				Children: make([]*Node, 0),
			},
		},
	}
}

// AddJob adds a job node to the tree.
func (et *ExecutionTree) AddJob(job *model.Job) *TreeNode {
	et.mu.Lock()
	defer et.mu.Unlock()

	status := StatusPending
	if job.Nested {
		status = StatusConditional
	}

	node := &TreeNode{
		Node: &Node{
			Name:         job.Name,
			Status:       status,
			Children:     make([]*Node, 0),
			Dependencies: make([]string, 0),
		},
	}
	et.Root.Node.Children = append(et.Root.Node.Children, node.Node)
	return node
}

// AddJobWithDeps adds a job node to the tree with dependencies.
func (et *ExecutionTree) AddJobWithDeps(jobName string, deps []string) *TreeNode {
	et.mu.Lock()
	defer et.mu.Unlock()

	node := &TreeNode{
		Node: &Node{
			Name:         jobName,
			Status:       StatusPending,
			Children:     make([]*Node, 0),
			Dependencies: deps,
		},
	}
	et.Root.Node.Children = append(et.Root.Node.Children, node.Node)
	return node
}

// AddStep adds a step node to a job.
func (job *TreeNode) AddStep(stepName string) *TreeNode {
	job.mu.Lock()
	defer job.mu.Unlock()

	node := &TreeNode{
		Node: &Node{
			Name:   stepName,
			Status: StatusRunning,
		},
	}
	job.Node.Children = append(job.Node.Children, node.Node)
	return node
}

// AddStepDeferred adds a deferred step node to a job.
func (job *TreeNode) AddStepDeferred(stepName string) *TreeNode {
	job.mu.Lock()
	defer job.mu.Unlock()

	node := &TreeNode{
		Node: &Node{
			Name:     stepName,
			Status:   StatusRunning,
			Deferred: true,
		},
	}
	job.Node.Children = append(job.Node.Children, node.Node)
	return node
}

// SetStatus updates a node's status.
func (node *TreeNode) SetStatus(status Status) {
	node.Node.SetStatus(status)
}

// RenderTree renders the entire tree to a string (live rendering).
func (et *ExecutionTree) RenderTree() string {
	et.mu.Lock()
	defer et.mu.Unlock()

	renderer := NewRenderer()
	return renderer.Render(et.Root.Node)
}

// CountLines returns the number of lines the tree will render.
func (et *ExecutionTree) CountLines() int {
	et.mu.Lock()
	defer et.mu.Unlock()

	return CountLines(et.Root.Node)
}

// GetChildren returns the children of a node.
func (node *TreeNode) GetChildren() []*TreeNode {
	children := node.Node.GetChildren()
	result := make([]*TreeNode, len(children))
	for i, child := range children {
		result[i] = &TreeNode{Node: child}
	}
	return result
}

// GetName returns the name of the node.
func (node *TreeNode) GetName() string {
	return node.Node.Name
}

// GetStatus returns the status of the node.
func (node *TreeNode) GetStatus() Status {
	return node.Node.Status
}
