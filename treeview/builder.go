package treeview

import (
	"fmt"

	"github.com/titpetric/atkins-ci/model"
)

// Builder constructs tree nodes from pipeline data.
type Builder struct {
	root *Node
}

// NewBuilder creates a new tree builder.
func NewBuilder(pipelineName string) *Builder {
	return &Builder{
		root: NewNode(pipelineName),
	}
}

// Root returns the root node.
func (b *Builder) Root() *Node {
	return b.root
}

// AddJob adds a job node to the tree with all its steps.
func (b *Builder) AddJob(job *model.Job, deps []string, jobName string) *TreeNode {
	// Create job node
	jobNode := NewJobNode(jobName, job.Nested)
	jobNode.Dependencies = deps
	jobNode.Summarize = job.Summarize

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

// AddJobWithoutSteps adds a job node to the tree without steps (steps should be added manually afterwards).
func (b *Builder) AddJobWithoutSteps(deps []string, jobName string, nested bool) *TreeNode {
	// Create job node
	jobNode := NewJobNode(jobName, nested)
	jobNode.Dependencies = deps

	b.root.AddChild(jobNode)

	return &TreeNode{
		Node: jobNode,
	}
}

// AddJobWithSummary adds a job node to the tree with summarization enabled.
func (b *Builder) AddJobWithSummary(job *model.Job, deps []string, jobName string) *TreeNode {
	jobNode := NewJobNode(jobName, job.Nested)
	jobNode.Dependencies = deps
	jobNode.Summarize = true

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
	cmd := step.String()

	// Build the name with annotations
	// Note: (deferred) is added by the renderer if node.Deferred is true
	stepName := cmd
	if step.If != "" {
		stepName = stepName + " (conditional)"
	}

	stepNode := NewNode(stepName)
	stepNode.Summarize = step.Summarize
	stepNode.Deferred = step.Deferred

	return stepNode
}

// BuildFromPipeline constructs a complete tree from a pipeline.
// Returns the root node ready to be rendered.
func BuildFromPipeline(pipeline *model.Pipeline, resolveDeps func(map[string]*model.Job, string) ([]string, error)) (*Node, error) {
	jobs := pipeline.Jobs
	if len(jobs) == 0 {
		jobs = pipeline.Tasks
	}

	builder := NewBuilder(pipeline.Name)

	// Get jobs in dependency order
	jobOrder, err := resolveDeps(jobs, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve job dependencies: %w", err)
	}

	// Convert jobOrder to a set for quick lookup
	willRun := make(map[string]bool)
	for _, jobName := range jobOrder {
		willRun[jobName] = true
	}

	// Build tree structure for static display - include all jobs
	// Sort job names by depth, then alphabetically for consistent display
	var jobNames []string
	for jobName := range jobs {
		jobNames = append(jobNames, jobName)
	}
	jobNames = SortJobsByDepth(jobNames)

	for _, jobName := range jobNames {
		job := jobs[jobName]
		jobNode := builder.AddJob(job, job.DependsOn, jobName)

		// Mark jobs that won't be executed
		if !willRun[jobName] {
			jobNode.Node.Status = StatusSkipped
		}
	}

	return builder.Root(), nil
}
