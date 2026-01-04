package treeview

import (
	"fmt"

	"github.com/titpetric/atkins-ci/model"
)

// Builder constructs tree nodes from pipeline data
type Builder struct {
	root *Node
}

// NewBuilder creates a new tree builder
func NewBuilder(pipelineName string) *Builder {
	return &Builder{
		root: NewNode(pipelineName),
	}
}

// Root returns the root node
func (b *Builder) Root() *Node {
	return b.root
}

// AddJob adds a job node to the tree with all its steps
func (b *Builder) AddJob(jobName string, job *model.Job, deps []string) *TreeNode {
	// Create job node
	jobNode := NewJobNode(jobName, job.Nested)
	jobNode.Dependencies = deps

	// Add steps as children
	for _, step := range job.Steps {
		stepNode := b.buildStepNode(step)
		jobNode.AddChild(stepNode)
	}

	b.root.AddChild(jobNode)

	return &TreeNode{
		Node: jobNode,
	}
}

// buildStepNode constructs a step node from a step definition
func (b *Builder) buildStepNode(step *model.Step) *Node {
	// Build step command/label
	cmd := b.getStepCommand(step)

	// Build the name with annotations
	// Note: (deferred) is added by the renderer if node.Deferred is true
	stepName := cmd
	if step.For != "" {
		stepName = stepName + " (waiting)"
	}
	if step.If != "" {
		stepName = stepName + " (conditional)"
	}

	stepNode := NewNode(stepName)
	stepNode.Deferred = step.Defer != "" || step.Deferred

	return stepNode
}

// getStepCommand extracts the command from a step
func (b *Builder) getStepCommand(step *model.Step) string {
	if step.Defer != "" {
		// Deferred step - show the defer command
		return step.Defer
	} else if step.Run != "" {
		return step.Run
	} else if step.Cmd != "" {
		return step.Cmd
	} else if len(step.Cmds) > 0 {
		// For cmds array, just show the first one as a placeholder
		return step.Cmds[0]
	} else if step.Name != "" {
		return step.Name
	}
	return ""
}

// BuildFromPipeline constructs a complete tree from a pipeline
// Returns the root node ready to be rendered
func BuildFromPipeline(pipeline *model.Pipeline, resolveDeps func(map[string]*model.Job, string) ([]string, error)) (*Node, error) {
	allJobs := pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = pipeline.Tasks
	}

	builder := NewBuilder(pipeline.Name)

	// Get jobs in dependency order
	jobOrder, err := resolveDeps(allJobs, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve job dependencies: %w", err)
	}

	// Build tree structure for static display
	for _, jobName := range jobOrder {
		job := allJobs[jobName]

		// Add dependencies (will be handled by resolveDeps function caller)
		deps := make([]string, 0)

		builder.AddJob(jobName, job, deps)
	}

	return builder.Root(), nil
}
