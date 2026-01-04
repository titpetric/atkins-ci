package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/spinner"
	"github.com/titpetric/atkins-ci/treeview"
	"golang.org/x/sync/errgroup"
)

// TreeRenderer manages in-place tree rendering with ANSI cursor control
// Kept for backward compatibility, wraps treeview.Display
type TreeRenderer struct {
	display *treeview.Display
	mu      sync.Mutex
}

// NewTreeRenderer creates a new tree renderer
func NewTreeRenderer() *TreeRenderer {
	return &TreeRenderer{
		display: treeview.NewDisplay(),
	}
}

// Render outputs the tree, updating in-place if previously rendered
func (tr *TreeRenderer) Render(tree *treeview.ExecutionTree) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.display.Render(tree.Root.Node)
}

// Options provides configuration for the executor
type Options struct {
	DefaultTimeout time.Duration
}

// DefaultOptions returns the default executor options
func DefaultOptions() *Options {
	return &Options{
		DefaultTimeout: 300 * time.Second, // 5 minutes default
	}
}

// Executor runs pipeline jobs and steps
type Executor struct {
	opts *Options
}

// NewExecutor creates a new executor with default options
func NewExecutor() *Executor {
	return &Executor{
		opts: DefaultOptions(),
	}
}

// NewExecutorWithOptions creates a new executor with custom options
func NewExecutorWithOptions(opts *Options) *Executor {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Executor{
		opts: opts,
	}
}

// parseTimeout parses a timeout string into a duration, using default if empty
func parseTimeout(timeoutStr string, defaultTimeout time.Duration) time.Duration {
	if timeoutStr == "" {
		return defaultTimeout
	}
	duration, err := time.ParseDuration(timeoutStr)
	if err != nil {
		// If parsing fails, return default
		return defaultTimeout
	}
	return duration
}

// ExecuteJob runs a single job
func (e *Executor) ExecuteJob(parentCtx context.Context, ctx *ExecutionContext, jobName string, job *model.Job) error {
	// Parse job timeout
	jobTimeout := parseTimeout(job.Timeout, e.opts.DefaultTimeout)

	// Create a child context with the job timeout
	jobCtx, cancel := context.WithTimeout(parentCtx, jobTimeout)
	defer cancel()

	// Store context in execution context for use in steps
	ctx.Context = jobCtx

	// Merge job variables into context
	if job.Vars != nil {
		for k, v := range job.Vars {
			ctx.Variables[k] = v
		}
	}

	// Merge job environment
	if job.Env != nil {
		for k, v := range job.Env {
			ctx.Env[k] = v
		}
	}

	// Execute steps
	if len(job.Steps) > 0 {
		return e.executeSteps(jobCtx, ctx, job.Steps)
	}

	// Execute legacy cmd/cmds format
	if job.Run != "" {
		return e.executeCommand(jobCtx, ctx, job.Run)
	}

	if job.Cmd != "" {
		return e.executeCommand(jobCtx, ctx, job.Cmd)
	}

	if len(job.Cmds) > 0 {
		for _, cmd := range job.Cmds {
			if err := e.executeCommand(jobCtx, ctx, cmd); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

// executeSteps runs a sequence of steps (deferred steps are already at the end of the list)
func (e *Executor) executeSteps(jobCtx context.Context, execCtx *ExecutionContext, steps []*model.Step) error {
	eg := new(errgroup.Group)
	detached := 0

	for idx, step := range steps {
		if step.Detach {
			detached++
			eg.Go(func() error {
				return e.executeStep(jobCtx, execCtx, steps[idx], idx)
			})
			continue
		}
		if err := e.executeStep(jobCtx, execCtx, steps[idx], idx); err != nil {
			return err
		}
	}

	if detached > 0 {
		return eg.Wait()
	}

	return nil
}

// executeStep runs a single step
func (e *Executor) executeStep(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepIndex int) error {
	// Handle step-level environment variables
	stepCtx := &ExecutionContext{
		Variables: execCtx.Variables,
		Env:       make(map[string]string),
		Results:   execCtx.Results,
		QuietMode: execCtx.QuietMode,
		Pipeline:  execCtx.Pipeline,
		Job:       execCtx.Job,
		Step:      step,
		Depth:     execCtx.Depth + 1,
		Context:   jobCtx,
	}

	// Copy parent env and add step-specific env
	for k, v := range execCtx.Env {
		stepCtx.Env[k] = v
	}
	if step.Env != nil {
		for k, v := range step.Env {
			stepCtx.Env[k] = v
		}
	}

	// Get step node from tree
	var stepNode *treeview.Node
	if jobNode := execCtx.CurrentJob; jobNode != nil {
		children := jobNode.GetChildren()
		if stepIndex < len(children) {
			stepNode = children[stepIndex].Node
		}
	}

	// Evaluate if condition
	shouldRun, err := EvaluateIf(stepCtx)
	if err != nil {
		// If condition evaluation fails, skip the step
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusSkipped)
		}
		return fmt.Errorf("failed to evaluate if condition for step %q: %w", step.Name, err)
	}

	if !shouldRun {
		// Mark step as skipped
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusSkipped)
		}
		return nil
	}

	// Handle for loop expansion
	if step.For != "" {
		return e.executeStepWithForLoop(jobCtx, execCtx, step, stepIndex, stepNode)
	}

	// Determine which command to run
	var cmd string
	if step.Run != "" {
		cmd = step.Run
	} else if step.Cmd != "" {
		cmd = step.Cmd
	} else if len(step.Cmds) > 0 {
		cmd = strings.Join(step.Cmds, " && ")
	} else {
		return nil
	}

	// Execute single iteration of the step
	return e.executeStepIteration(jobCtx, execCtx, step, stepNode, cmd)
}

// executeStepWithForLoop handles for loop expansion and execution
// Each iteration becomes a separate execution with iteration variables overlaid on context
func (e *Executor) executeStepWithForLoop(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, _ int, stepNode *treeview.Node) error {
	// Expand the for loop to get all iterations
	iterations, err := ExpandFor(execCtx, ExecuteCommand)
	if err != nil {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("failed to expand for loop for step %q: %w", step.Name, err)
	}

	if len(iterations) == 0 {
		// Empty for loop - mark as passed
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusPassed)
		}
		execCtx.StepsCount++
		execCtx.StepsPassed++
		return nil
	}

	// Add sub-nodes for each iteration to expand the tree
	iterationNodes := make([]*treeview.Node, 0, len(iterations))
	if stepNode != nil {
		for i := range iterations {
			iterName := fmt.Sprintf("[%d]", i+1)
			iterNode := &treeview.Node{
				Name:   iterName,
				Status: treeview.StatusPending,
			}
			stepNode.Children = append(stepNode.Children, iterNode)
			iterationNodes = append(iterationNodes, iterNode)
		}
	}

	// Render tree with expanded iterations
	execCtx.Display.Render(execCtx.CurrentStep)

	// Execute each iteration
	var lastErr error
	for idx, iteration := range iterations {
		// Create iteration context by overlaying iteration variables on parent context
		iterCtx := &ExecutionContext{
			Variables:   copyVariables(execCtx.Variables),
			Env:         execCtx.Env,
			Results:     execCtx.Results,
			QuietMode:   execCtx.QuietMode,
			Pipeline:    execCtx.Pipeline,
			Job:         execCtx.Job,
			Step:        execCtx.Step,
			Depth:       execCtx.Depth,
			Builder:     execCtx.Builder,
			CurrentJob:  execCtx.CurrentJob,
			CurrentStep: execCtx.CurrentStep,
			Context:     jobCtx,
		}

		// Overlay iteration variables (they override parent variables)
		for k, v := range iteration.Variables {
			iterCtx.Variables[k] = v
		}

		// Determine which command to run
		var cmd string
		if step.Run != "" {
			cmd = step.Run
		} else if step.Cmd != "" {
			cmd = step.Cmd
		} else if len(step.Cmds) > 0 {
			cmd = strings.Join(step.Cmds, " && ")
		} else {
			continue // Skip if no command
		}

		// Get the iteration sub-node
		var iterNode *treeview.Node
		if len(iterationNodes) > idx {
			iterNode = iterationNodes[idx]
		}

		// Execute this iteration with the iteration sub-node
		if err := e.executeStepIteration(jobCtx, iterCtx, step, iterNode, cmd); err != nil {
			lastErr = err
			// Continue to next iteration even on error (collect all failures)
			// This matches yamlexpr behavior of processing all items
		}
	}

	if lastErr != nil {
		return lastErr
	}

	execCtx.StepsCount++
	execCtx.StepsPassed++
	return nil
}

// executeStepIteration executes a single step (or iteration of a step) with the given context
func (e *Executor) executeStepIteration(jobCtx context.Context, stepCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, cmd string) error {
	// Mark step as running
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusRunning)
	}

	// Start spinner and execute command
	s := spinner.New()
	s.Start()

	// Channel to signal command completion
	cmdDone := make(chan error)
	go func() {
		cmdDone <- e.executeCommand(jobCtx, stepCtx, cmd)
		close(cmdDone)
	}()

	// Update spinner in tree while command runs
	tickerTicker := time.NewTicker(100 * time.Millisecond)
	defer tickerTicker.Stop()

	for {
		select {
		case err := <-cmdDone:
			s.Stop()
			tickerTicker.Stop()

			// Update tree node status
			if stepNode != nil {
				if err != nil {
					stepNode.SetStatus(treeview.StatusFailed)
					return err
				}
				stepNode.SetStatus(treeview.StatusPassed)
				// Clear error log on successful step
				ErrorLogMutex.Lock()
				ErrorLog.Reset()
				ErrorLogMutex.Unlock()
			}

			// Render tree with final state
			stepCtx.Display.Render(stepCtx.Builder.Root())
			return nil

		case <-tickerTicker.C:
			if stepNode != nil {
				stepNode.SetSpinner(s.String())

				stepCtx.Display.Render(stepCtx.Builder.Root())
			}
		}
	}
}

// copyVariables creates a shallow copy of a variables map
func copyVariables(vars map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for k, v := range vars {
		copy[k] = v
	}
	return copy
}

// countOutputLines counts the number of newlines in output
func countOutputLines(output string) int {
	count := 0
	for _, ch := range output {
		if ch == '\n' {
			count++
		}
	}
	return count
}

// executeCommand runs a single command with interpolation and respects context timeout
func (e *Executor) executeCommand(ctx context.Context, execCtx *ExecutionContext, cmd string) error {
	// Interpolate the command
	interpolated, err := InterpolateCommand(cmd, execCtx)
	if err != nil {
		return fmt.Errorf("interpolation failed: %w", err)
	}

	// Check if context is already cancelled
	if ctx != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("command execution cancelled or timed out: %w", ctx.Err())
		default:
		}
	}

	// Execute the command via bash with quiet mode
	output, err := ExecuteCommandWithQuiet(interpolated, execCtx.QuietMode)
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	// Only print output if not in quiet mode (quiet mode 1 = suppress output)
	if execCtx.QuietMode == 0 && output != "" {
		fmt.Print(output)
	}

	return nil
}
