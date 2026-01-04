package runner

import (
	"context"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/treeview"
)

// IterationContext holds the variables for a single iteration of a for loop
type IterationContext struct {
	Variables map[string]interface{}
}

// ExecutionContext holds runtime state during pipeline Exec
type ExecutionContext struct {
	Context context.Context

	Variables map[string]interface{}
	Env       map[string]string
	Results   map[string]interface{}

	Verbose bool

	Pipeline *model.Pipeline
	Job      *model.Job
	Step     *model.Step

	Depth       int // Nesting depth for indentation
	StepsCount  int // Total number of steps executed
	StepsPassed int // Number of steps that passed

	CurrentJob  *treeview.TreeNode
	CurrentStep *treeview.Node

	Display *treeview.Display
	Builder *treeview.Builder
}
