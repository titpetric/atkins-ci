package treeview

import (
	"sync"
	"time"

	"github.com/titpetric/atkins/colors"
)

func (n *Node) StatusColor() string {
	var (
		haveChildren = n.HasChildren()
		haveDeps     = len(n.Dependencies) > 0
	)

	status := n.Status.String()
	if status == "" && (haveChildren || haveDeps) {
		return colors.Green("●")
	}
	return status
}

func (n *Node) Label() string {
	var (
		haveChildren = n.HasChildren()
		haveDeps     = len(n.Dependencies) > 0
	)

	name := n.Name //        = fmt.Sprintf("%s, [summarize %v]", n.Name, n.Summarize)

	switch n.Status {
	case StatusRunning:
		if haveChildren {
			return colors.BrightOrange(name)
		}
		return colors.White(name)
	case StatusPassed:
		return colors.BrightWhite(name)
	case StatusFailed:
		return colors.BrightRed(name)
	case StatusSkipped:
		return colors.BrightYellow(name)
	case StatusConditional:
		return colors.BrightYellow(name)
	default:
		if haveChildren || haveDeps {
			return colors.BrightOrange(name)
		}
	}
	return colors.White(name)
}

// Node represents a node in the tree (job, step, or iteration).
type Node struct {
	Name         string
	ID           string // Unique identifier (e.g., "job.steps.0", "job.steps.1" for iterations)
	Status       Status
	UpdatedAt    time.Time
	Children     []*Node
	Dependencies []string
	Deferred     bool
	Summarize    bool
	Output       []string // Multi-line output from command execution
	mu           sync.Mutex
}

// SetStatus updates a node's status thread-safely.
func (n *Node) SetStatus(status Status) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
	n.Deferred = false
	n.UpdatedAt = time.Now()
}

// SetOutput sets the output lines for this node (from command execution).
func (n *Node) SetOutput(lines []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Output = lines
}

// AddChild adds a child node.
func (n *Node) AddChild(child *Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Children = append(n.Children, child)
}

// AddChildren adds multiple child nodes.
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

// GetChildren returns a copy of the children slice (thread-safe).
func (n *Node) GetChildren() []*Node {
	n.mu.Lock()
	defer n.mu.Unlock()
	children := make([]*Node, len(n.Children))
	copy(children, n.Children)
	return children
}

// NewNode creates a new tree node.
func NewNode(name string) *Node {
	return &Node{
		Name:         name,
		Status:       StatusPending,
		UpdatedAt:    time.Now(),
		Children:     make([]*Node, 0),
		Dependencies: make([]string, 0),
	}
}

// NewJobNode creates a new job node.
func NewJobNode(name string, nested bool) *Node {
	node := NewNode(name)
	if nested {
		node.Status = StatusConditional
	}
	return node
}

// NewStepNode creates a new step node.
func NewStepNode(name string, deferred bool) *Node {
	node := NewNode(name)
	node.Status = StatusRunning
	node.Deferred = deferred
	return node
}
