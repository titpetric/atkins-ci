package runner

import (
	"context"
	"sync"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/treeview"
)

// IterationContext holds the variables for a single iteration of a for loop.
type IterationContext struct {
	Variables map[string]interface{}
}

// ExecutionContext holds runtime state during pipeline Exec.
type ExecutionContext struct {
	Context context.Context

	Env     map[string]string
	Results map[string]interface{}
	Verbose bool

	Variables map[string]interface{}

	Pipeline *model.Pipeline
	Job      *model.Job
	Step     *model.Step

	Depth       int // Nesting depth for indentation
	StepsCount  int // Total number of steps executed
	StepsPassed int // Number of steps that passed

	CurrentJob  *treeview.TreeNode
	CurrentStep *treeview.Node

	Display  *treeview.Display
	Builder  *treeview.Builder
	JobNodes map[string]*treeview.TreeNode // Map of job names to their tree nodes
	Logger   *StepLogger

	// Sequential step counter for this job (incremented for each step execution)
	StepSequence int
	stepSeqMu    sync.Mutex
}

// Copy copies everything except Context and Variables.
func (e *ExecutionContext) Copy() *ExecutionContext {
	return &ExecutionContext{
		Env:          e.Env,
		Results:      e.Results,
		Verbose:      e.Verbose,
		Pipeline:     e.Pipeline,
		Job:          e.Job,
		Step:         e.Step,
		Depth:        e.Depth + 1,
		StepsCount:   e.StepsCount,
		StepsPassed:  e.StepsPassed,
		CurrentJob:   e.CurrentJob,
		CurrentStep:  e.CurrentStep,
		Display:      e.Display,
		Builder:      e.Builder,
		JobNodes:     e.JobNodes,
		Logger:       e.Logger,
		StepSequence: e.StepSequence,
	}
}

// Render() will refresh the treeview.
func (e *ExecutionContext) Render() {
	e.Display.Render(e.Builder.Root())
}

// NextStepIndex returns the next sequential step index for this job execution.
// This ensures each step/iteration gets a unique number.
func (e *ExecutionContext) NextStepIndex() int {
	e.stepSeqMu.Lock()
	defer e.stepSeqMu.Unlock()
	idx := e.StepSequence
	e.StepSequence++
	return idx
}
