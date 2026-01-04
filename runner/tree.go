package runner

import (
	"strings"
	"sync"
	"time"

	"github.com/titpetric/atkins-ci/colors"
)

// NodeStatus represents the execution status of a node
type NodeStatus int

const (
	StatusPending NodeStatus = iota
	StatusRunning
	StatusPassed
	StatusFailed
)

// TreeNode represents a node in the execution tree
type TreeNode struct {
	Name         string
	Status       NodeStatus
	UpdatedAt    time.Time
	Spinner      string
	Children     []*TreeNode
	Dependencies []string
	Deferred     bool
	mu           sync.Mutex
}

// ExecutionTree holds the entire execution tree
type ExecutionTree struct {
	Root *TreeNode
	mu   sync.Mutex
}

// NewExecutionTree creates a new execution tree with a root node
func NewExecutionTree(pipelineName string) *ExecutionTree {
	return &ExecutionTree{
		Root: &TreeNode{
			Name:     pipelineName,
			Status:   StatusRunning,
			Children: make([]*TreeNode, 0),
		},
	}
}

// AddJob adds a job node to the tree
func (et *ExecutionTree) AddJob(jobName string) *TreeNode {
	et.mu.Lock()
	defer et.mu.Unlock()

	node := &TreeNode{
		Name:         jobName,
		Status:       StatusPending,
		Children:     make([]*TreeNode, 0),
		Dependencies: make([]string, 0),
	}
	et.Root.Children = append(et.Root.Children, node)
	return node
}

// AddJobWithDeps adds a job node to the tree with dependencies
func (et *ExecutionTree) AddJobWithDeps(jobName string, deps []string) *TreeNode {
	et.mu.Lock()
	defer et.mu.Unlock()

	node := &TreeNode{
		Name:         jobName,
		Status:       StatusPending,
		Children:     make([]*TreeNode, 0),
		Dependencies: deps,
	}
	et.Root.Children = append(et.Root.Children, node)
	return node
}

// AddStep adds a step node to a job
func (job *TreeNode) AddStep(stepName string) *TreeNode {
	job.mu.Lock()
	defer job.mu.Unlock()

	node := &TreeNode{
		Name:   stepName,
		Status: StatusRunning,
	}
	job.Children = append(job.Children, node)
	return node
}

// AddStepDeferred adds a deferred step node to a job
func (job *TreeNode) AddStepDeferred(stepName string) *TreeNode {
	job.mu.Lock()
	defer job.mu.Unlock()

	node := &TreeNode{
		Name:     stepName,
		Status:   StatusRunning,
		Deferred: true,
	}
	job.Children = append(job.Children, node)
	return node
}

// SetStatus updates a node's status
func (node *TreeNode) SetStatus(status NodeStatus) {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.Status = status
	node.UpdatedAt = time.Now()
}

// SetSpinner updates the spinner display
func (node *TreeNode) SetSpinner(spinner string) {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.Spinner = spinner
}

// RenderTree renders the entire tree to a string (live rendering)
func (et *ExecutionTree) RenderTree() string {
	et.mu.Lock()
	defer et.mu.Unlock()

	output := colors.BrightGreen(et.Root.Name) + "\n"
	for i, job := range sortChildrenByStatus(et.Root.Children) {
		isLast := i == len(et.Root.Children)-1
		output += renderNode(job, "", isLast)
	}
	return output
}

// CountLines returns the number of lines the tree will render
func (et *ExecutionTree) CountLines() int {
	et.mu.Lock()
	defer et.mu.Unlock()

	count := 1 // root line
	for _, job := range et.Root.Children {
		count += countNodeLines(job)
	}
	return count
}

func countNodeLines(node *TreeNode) int {
	count := 1 // this node
	for _, child := range node.Children {
		count += countNodeLines(child)
	}
	return count
}

func renderNode(node *TreeNode, prefix string, isLast bool) string {
	output := ""

	// Determine branch character
	branch := "├─ "
	if isLast {
		branch = "└─ "
	}

	// Determine status indicator and color
	var status string
	var nameColor string
	isJob := len(node.Children) > 0
	
	switch node.Status {
	case StatusPassed:
		status = colors.BrightGreen("✓")
		if isJob {
			nameColor = colors.BrightGreen(node.Name)
		} else {
			nameColor = colors.BrightGreen(node.Name)
		}
	case StatusFailed:
		status = colors.BrightRed("✗")
		nameColor = colors.BrightRed(node.Name)
	case StatusRunning:
		if isJob {
			// Job in orange
			nameColor = colors.BrightOrange(node.Name)
		} else {
			// Step in white
			nameColor = colors.White(node.Name)
		}
		if node.Spinner != "" {
			status = node.Spinner
		} else {
			status = ""
		}
	default:
		// Pending/future items
		nameColor = colors.Gray(node.Name)
		status = ""
	}

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
	if len(node.Children) > 0 {
		// Determine continuation character
		continuation := "│  "
		if isLast {
			continuation = "   "
		}

		// Sort children by status: completed first, then running, then pending
		sortedChildren := sortChildrenByStatus(node.Children)

		for j, child := range sortedChildren {
			childIsLast := j == len(sortedChildren)-1
			output += renderNode(child, prefix+continuation, childIsLast)
		}
	}

	return output
}

// sortChildrenByStatus reorders children so completed items appear first
// Order: Passed, Failed, Running, Pending
func sortChildrenByStatus(children []*TreeNode) []*TreeNode {
	sorted := make([]*TreeNode, len(children))
	copy(sorted, children)

	// Stable sort to maintain order within each status group
	// Use bubble sort (stable) to sort by status priority
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Status != sorted[j+1].Status {
				if statusPriority(sorted[j].Status) < statusPriority(sorted[j+1].Status) {
					sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
				}
				continue
			}
			a, b := time.Since(sorted[j].UpdatedAt), time.Since(sorted[j+1].UpdatedAt)
			if a < b {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// statusPriority returns a priority number for sorting (higher = comes first)
func statusPriority(status NodeStatus) int {
	switch status {
	case StatusPassed:
		return 4
	case StatusFailed:
		return 3
	case StatusRunning:
		return 2
	case StatusPending:
		return 1
	default:
		return 0
	}
}
