package treeview

import (
	"sync"
	"time"
)

// Status represents the execution status of a node
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusPassed
	StatusFailed
	StatusSkipped
	StatusConditional
)

// Node represents a node in the tree (job, step, or iteration)
type Node struct {
	Name         string
	Status       Status
	UpdatedAt    time.Time
	Spinner      string
	Children     []*Node
	Dependencies []string
	Deferred     bool
	mu           sync.Mutex
}

// SetStatus updates a node's status thread-safely
func (n *Node) SetStatus(status Status) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
	n.UpdatedAt = time.Now()
}

// SetSpinner updates the spinner display thread-safely
func (n *Node) SetSpinner(spinner string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Spinner = spinner
}

// AddChild adds a child node
func (n *Node) AddChild(child *Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Children = append(n.Children, child)
}

// AddChildren adds multiple child nodes
func (n *Node) AddChildren(children ...*Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Children = append(n.Children, children...)
}

// HasChildren returns true or false if the node has children.
func (n *Node) HasChildren() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return len(n.Children) > 0
}

// GetChildren returns a copy of the children slice (thread-safe)
func (n *Node) GetChildren() []*Node {
	n.mu.Lock()
	defer n.mu.Unlock()
	children := make([]*Node, len(n.Children))
	copy(children, n.Children)
	return children
}

// NewNode creates a new tree node
func NewNode(name string) *Node {
	return &Node{
		Name:         name,
		Status:       StatusPending,
		UpdatedAt:    time.Now(),
		Children:     make([]*Node, 0),
		Dependencies: make([]string, 0),
	}
}

// NewJobNode creates a new job node
func NewJobNode(name string, nested bool) *Node {
	node := NewNode(name)
	if nested {
		node.Status = StatusConditional
	}
	return node
}

// NewStepNode creates a new step node
func NewStepNode(name string, deferred bool) *Node {
	node := NewNode(name)
	node.Status = StatusRunning
	node.Deferred = deferred
	return node
}
