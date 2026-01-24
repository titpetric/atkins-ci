package runner

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/titpetric/atkins/eventlog"
	"github.com/titpetric/atkins/model"
	"github.com/titpetric/atkins/treeview"
)

// LineCapturingWriter captures all output written to it.
type LineCapturingWriter struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

// NewLineCapturingWriter creates a new LineCapturingWriter.
func NewLineCapturingWriter() *LineCapturingWriter {
	return &LineCapturingWriter{}
}

// Write implements io.Writer.
func (w *LineCapturingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

// GetLines returns all captured output as lines.
func (w *LineCapturingWriter) GetLines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	output := w.buffer.String()
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	// Remove the last empty line if it exists (from final newline)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// String returns the raw captured output.
func (w *LineCapturingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

// Options provides configuration for the executor.
type Options struct {
	DefaultTimeout time.Duration
}

// DefaultOptions returns the default executor options.
func DefaultOptions() *Options {
	return &Options{
		DefaultTimeout: 300 * time.Second, // 5 minutes default
	}
}

// Executor runs pipeline jobs and steps.
type Executor struct {
	opts *Options
}

// NewExecutor creates a new executor with default options.
func NewExecutor() *Executor {
	return &Executor{
		opts: DefaultOptions(),
	}
}

// NewExecutorWithOptions creates a new executor with custom options.
func NewExecutorWithOptions(opts *Options) *Executor {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Executor{
		opts: opts,
	}
}

// executeCommands executes a list of commands, updating child nodes if available.
// Returns the last error encountered (continues on errors to collect all failures).
func (e *Executor) executeCommands(ctx context.Context, stepCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, commands []string, stepIndex int) error {
	if len(commands) == 0 {
		return nil
	}

	// If step has multiple commands, update child nodes individually
	var cmdNodes []*treeview.Node
	if stepNode != nil {
		cmdNodes = stepNode.GetChildren()
	}

	var lastErr error
	for i, cmd := range commands {
		var cmdNode *treeview.Node
		if i < len(cmdNodes) {
			cmdNode = cmdNodes[i]
		} else if stepNode != nil {
			cmdNode = stepNode // Fallback to parent if no child nodes
		}
		if err := e.executeStepIteration(ctx, stepCtx, step, cmdNode, cmd, stepIndex+i); err != nil {
			lastErr = err
		}
	}

	// Update parent node status if we used child nodes
	if len(cmdNodes) > 0 && stepNode != nil {
		if lastErr != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		} else {
			stepNode.SetStatus(treeview.StatusPassed)
		}
	}

	return lastErr
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

// ExecuteJob runs a single job.
func (e *Executor) ExecuteJob(parentCtx context.Context, execCtx *ExecutionContext) error {
	if execCtx == nil {
		return fmt.Errorf("execution context is nil")
	}

	job := execCtx.Job
	if job == nil {
		return fmt.Errorf("job is nil in execution context")
	}

	// Parse job timeout
	jobTimeout := parseTimeout(job.Timeout, e.opts.DefaultTimeout)

	// Create a child context with the job timeout
	ctx, cancel := context.WithTimeout(parentCtx, jobTimeout)
	defer cancel()

	// Store context in execution context for use in steps
	execCtx.Context = ctx

	// Merge job variables into context with interpolation
	if err := MergeVariables(job.Decl, execCtx); err != nil {
		return err
	}

	// Execute steps
	steps := job.Children()
	return e.executeSteps(ctx, execCtx, steps)
}

// executeSteps runs a sequence of steps (deferred steps are already at the end of the list)
func (e *Executor) executeSteps(ctx context.Context, execCtx *ExecutionContext, steps []*model.Step) error {
	eg := new(errgroup.Group)

	detached := 0
	deferredSteps := []*model.Step{}
	deferredIndices := []int{}

	// Wait for all detached steps to complete before running deferred steps.
	wait := func() error {
		if detached > 0 {
			if err := eg.Wait(); err != nil {
				return err
			}
			detached = 0
		}
		return nil
	}

	// First pass: execute non-detached steps and collect deferred steps
	for idx, step := range steps {
		if step.IsDeferred() {
			// Collect deferred steps for later execution
			deferredSteps = append(deferredSteps, step)
			deferredIndices = append(deferredIndices, idx)
			continue
		}

		if step.Detach {
			detached++
			eg.Go(func() error {
				return e.executeStep(ctx, execCtx, steps[idx], idx)
			})
			continue
		}

		if err := wait(); err != nil {
			return err
		}

		if err := e.executeStep(ctx, execCtx, steps[idx], idx); err != nil {
			return err
		}
	}

	if err := wait(); err != nil {
		return err
	}

	// Second pass: execute deferred steps after all detached steps are done
	for i, step := range deferredSteps {
		stepIdx := deferredIndices[i]

		// Find the deferred step node by looking for deferred nodes in the tree
		// We need to find it by matching deferred status, not by index (since for loops may have expanded)
		var stepNode *treeview.Node
		if execCtx.CurrentJob != nil {
			children := execCtx.CurrentJob.GetChildren()
			deferredCount := 0
			// Count deferred nodes to find the i-th deferred node
			for _, child := range children {
				if child.Node.Deferred {
					if deferredCount == i {
						stepNode = child.Node
						break
					}
					deferredCount++
				}
			}
		}

		if stepNode != nil {
			// Update status to running and re-render to show the transition
			stepNode.SetStatus(treeview.StatusRunning)

			// Execute step with the actual found node
			if err := e.executeStepWithNode(ctx, execCtx, step, stepNode); err != nil {
				return err
			}
		} else {
			// Fallback to executeStep if node not found
			if err := e.executeStep(ctx, execCtx, step, stepIdx); err != nil {
				return err
			}
		}
	}

	return nil
}

// executeStepWithNode runs a single step with a provided node
func (e *Executor) executeStepWithNode(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node) error {
	// Handle step-level environment variables
	stepCtx := execCtx.Copy()
	stepCtx.Context = ctx
	stepCtx.Step = step

	env := make(map[string]string)
	// Copy parent env
	for k, v := range execCtx.Env {
		env[k] = v
	}
	stepCtx.Env = env

	// Merge step-level env with interpolation
	if err := MergeVariables(step.Decl, stepCtx); err != nil {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("failed to process step env: %w", err)
	}

	stepCtx.CurrentStep = stepNode

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
		// Mark step as skipped and log it
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusSkipped)
			if step.If != "" {
				stepNode.SetIf(step.If)
			}
		}
		// Get step name for logging
		stepName := step.Name
		if stepName == "" && stepNode != nil {
			stepName = stepNode.Name
		}
		// Get sequential step index from the parent execution context
		seqIndex := execCtx.NextStepIndex()
		// Log SKIP event
		jobName := ""
		if execCtx.Job != nil {
			jobName = execCtx.Job.Name
		}
		stepID := generateStepID(jobName, seqIndex)
		if execCtx.EventLogger != nil {
			startOffset := execCtx.EventLogger.GetElapsed()
			execCtx.EventLogger.LogExec(eventlog.ResultSkipped, stepID, stepName, startOffset, 0, nil)
		}
		return nil
	}

	// Handle for loop expansion
	if step.For != "" {
		return e.executeStepWithForLoop(ctx, stepCtx, step, 0, stepNode)
	} else {
		// Handle task invocation
		if step.Task != "" {
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusRunning)
			}
			return e.executeTaskStep(ctx, stepCtx, step, stepNode)
		}
	}

	// Execute all commands
	return e.executeCommands(ctx, stepCtx, step, stepNode, step.Commands(), 0)
}

// executeStep runs a single step
func (e *Executor) executeStep(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepIndex int) error {
	defer execCtx.Render()

	// Get the next sequential step index from the PARENT context before copying
	// This ensures all steps in a job get unique sequential indices
	seqIndex := execCtx.NextStepIndex()

	// Handle step-level environment variables
	stepCtx := execCtx.Copy()
	stepCtx.Context = ctx
	stepCtx.Step = step
	stepCtx.StepSequence = seqIndex // Set the index for this step

	env := make(map[string]string)
	// Copy parent env
	for k, v := range execCtx.Env {
		env[k] = v
	}
	stepCtx.Env = env

	// Merge step-level env with interpolation (will be done after getting stepNode)
	// For now, deferred until we have the node reference

	// Get step node from tree
	var stepNode *treeview.Node
	if jobNode := execCtx.CurrentJob; jobNode != nil {
		children := jobNode.GetChildren()
		if stepIndex < len(children) {
			stepNode = children[stepIndex].Node
			stepCtx.CurrentStep = stepNode
		}
	}

	// Merge step-level env with interpolation
	if err := MergeVariables(step.Decl, stepCtx); err != nil {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("failed to process step env: %w", err)
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
		// Mark step as skipped and log it
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusSkipped)
			if step.If != "" {
				stepNode.SetIf(step.If)
			}
		}
		// Get step name for logging
		stepName := step.Name
		if stepName == "" && stepNode != nil {
			stepName = stepNode.Name
		}
		// Log SKIP event using pre-assigned seqIndex
		jobName := ""
		if execCtx.Job != nil {
			jobName = execCtx.Job.Name
		}
		stepID := generateStepID(jobName, seqIndex)
		if execCtx.EventLogger != nil {
			startOffset := execCtx.EventLogger.GetElapsed()
			execCtx.EventLogger.LogExec(eventlog.ResultSkipped, stepID, stepName, startOffset, 0, nil)
		}
		return nil
	}

	// Handle task invocation
	if step.Task != "" {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusRunning)
		}
		return e.executeTaskStep(ctx, stepCtx, step, stepNode)
	}

	// Handle for loop expansion
	if step.For != "" {
		stepNode.Summarize = step.Summarize
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusRunning)
		}
		if err := e.executeStepWithForLoop(ctx, stepCtx, step, stepIndex, stepNode); err != nil {
			stepNode.SetStatus(treeview.StatusFailed)
			return err
		}
		return nil
	}

	// Execute all commands
	return e.executeCommands(ctx, stepCtx, step, stepNode, step.Commands(), stepIndex)
}

// executeStepWithForLoop handles for loop expansion and execution
// Each iteration becomes a separate execution with iteration variables overlaid on context
func (e *Executor) executeStepWithForLoop(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepIndex int, stepNode *treeview.Node) error {
	// Expand the for loop to get all iterations
	exec := NewExecWithEnv(execCtx.Env)
	iterations, err := ExpandFor(execCtx, exec.ExecuteCommand)
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

	stepNode.Summarize = step.Summarize

	// Build iteration nodes as children of the step node
	iterationNodes := make([]*treeview.Node, 0, len(iterations))
	if stepNode != nil {
		// Get the command template
		var cmdTemplate string
		if step.Task == "" {
			cmdTemplate = step.String()
		}

		// Create node for each iteration with interpolated command
		for idx, iteration := range iterations {
			// Interpolate command with iteration variables
			iterCtx := execCtx.Copy()
			for k, v := range iteration.Variables {
				iterCtx.Variables[k] = v
			}

			var interpolated string
			var nodeName string
			// For task invocations, use the task name; otherwise interpolate the command
			if step.Task != "" {
				interpolated = step.Task
				nodeName = interpolated
			} else {
				var err error
				interpolated, err = InterpolateCommand(cmdTemplate, iterCtx)
				if err != nil {
					if stepNode != nil {
						stepNode.SetStatus(treeview.StatusFailed)
					}
					return fmt.Errorf("failed to interpolate command for iteration %d: %w", idx, err)
				}

				// Use the interpolated command as the node name
				nodeName = interpolated
			}

			// Get job name for ID generation
			jobName := ""
			if execCtx.Job != nil {
				jobName = execCtx.Job.Name
			}

			// Generate unique ID for this iteration
			iterSeqIndex := execCtx.StepSequence + idx
			iterID := fmt.Sprintf("jobs.%s.steps.%d", jobName, iterSeqIndex)

			iterNode := &treeview.Node{
				Name:      nodeName,
				ID:        iterID,
				Status:    treeview.StatusPending,
				Summarize: step.Summarize,
			}

			// If step has multiple commands, create child nodes for each command
			if len(step.Cmds) > 0 {
				for _, cmd := range step.Cmds {
					// Interpolate each command with iteration variables
					interpolatedCmd, err := InterpolateCommand(cmd, iterCtx)
					if err != nil {
						if stepNode != nil {
							stepNode.SetStatus(treeview.StatusFailed)
						}
						return fmt.Errorf("failed to interpolate command for iteration %d: %w", idx, err)
					}
					iterNode.AddChild(treeview.NewCmdNode(interpolatedCmd))
				}
			}

			// Add as child of the step node
			stepNode.AddChild(iterNode)
			iterationNodes = append(iterationNodes, iterNode)
		}
	}

	// Render tree with expanded iterations
	execCtx.Render()

	// Execute each iteration - use errgroup for detached (parallel) execution
	var eg *errgroup.Group
	if step.Detach {
		eg = new(errgroup.Group)
		eg.SetLimit(runtime.NumCPU())
	}

	var lastErr error
	var errMu sync.Mutex

	for idx, iteration := range iterations {
		idx := idx
		iteration := iteration

		executeIteration := func() error {
			// Create iteration context by overlaying iteration variables on parent context
			iterCtx := execCtx.Copy()
			iterCtx.Context = ctx

			// Overlay iteration variables (they override parent variables)
			for k, v := range iteration.Variables {
				iterCtx.Variables[k] = v
			}

			// Merge step-level env with interpolation
			// This needs to happen before building the command so env vars can be interpolated
			if err := MergeVariables(step.Decl, iterCtx); err != nil {
				return fmt.Errorf("failed to process step env for iteration %d: %w", idx, err)
			}

			// Get the iteration sub-node
			var iterNode *treeview.Node
			if len(iterationNodes) > idx {
				iterNode = iterationNodes[idx]
			}

			// Handle task invocation or command execution
			if step.Task != "" {
				// Task invocation with loop variables
				if err := e.executeTaskStep(ctx, iterCtx, step, iterNode); err != nil {
					return err
				}
			} else {
				// Execute all commands for this iteration
				if err := e.executeCommands(ctx, iterCtx, step, iterNode, step.Commands(), stepIndex); err != nil {
					return err
				}
			}
			return nil
		}

		if step.Detach {
			// Run iterations in parallel
			eg.Go(func() error {
				if err := executeIteration(); err != nil {
					errMu.Lock()
					lastErr = err
					errMu.Unlock()
					// Don't return error - continue collecting all failures
				}
				return nil
			})
		} else {
			// Run iterations sequentially
			if err := executeIteration(); err != nil {
				lastErr = err
				// Continue to next iteration even on error (collect all failures)
			}
		}
	}

	// Wait for all parallel iterations to complete
	if eg != nil {
		_ = eg.Wait()
	}

	if stepNode != nil {
		if lastErr != nil {
			stepNode.SetStatus(treeview.StatusFailed)
			return lastErr
		}
		stepNode.SetStatus(treeview.StatusPassed)
	}

	execCtx.StepsCount++
	execCtx.StepsPassed++
	return nil
}

// executeStepIteration executes a single step (or iteration of a step) with the given context
func (e *Executor) executeStepIteration(ctx context.Context, stepCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, cmd string, stepIndex int) error {
	// Get step name for logging
	stepName := step.Name
	if stepName == "" && stepNode != nil {
		stepName = stepNode.Name
	}

	// Use the pre-assigned step sequence from the context
	seqIndex := stepCtx.StepSequence

	// Build step ID for logging
	jobName := ""
	if stepCtx.Job != nil {
		jobName = stepCtx.Job.Name
	}
	stepID := generateStepID(jobName, seqIndex)

	// Capture start offset for event log
	var startOffset float64
	if stepCtx.EventLogger != nil {
		startOffset = stepCtx.EventLogger.GetElapsed()
	}

	// Track start time for duration
	startTime := time.Now()

	// Mark step as running and render immediately to show state transition
	if stepNode != nil {
		stepNode.ID = stepID
		stepNode.SetStartOffset(startOffset)
		stepNode.SetStatus(treeview.StatusRunning)
		stepCtx.Render()
	}

	// Handle cmds: if step has multiple commands and child nodes exist, execute each command individually
	err := e.executeCommand(ctx, stepCtx, step, cmd)

	// Calculate duration
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Update tree node status and log result
	if stepNode != nil {
		stepNode.SetDuration(duration.Seconds())
		if err != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		} else {
			stepNode.SetStatus(treeview.StatusPassed)
		}
	}

	// Log single execution event
	if stepCtx.EventLogger != nil {
		result := eventlog.ResultPass
		if err != nil {
			result = eventlog.ResultFail
		}
		stepCtx.EventLogger.LogExec(result, stepID, stepName, startOffset, durationMs, err)
	}

	stepCtx.Render()
	return err
}

// executeTaskStep executes a task/job from within a step
// Supports both simple task invocation and for loop task invocation with loop variables
func (e *Executor) executeTaskStep(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node) error {
	defer execCtx.Render()

	// Get the task name from the step
	taskName := step.Task

	// Find the task in the pipeline
	allJobs := execCtx.Pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = execCtx.Pipeline.Tasks
	}

	taskJob, exists := allJobs[taskName]
	if !exists {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("task %q not found in pipeline", taskName)
	}

	// Execute dependencies first (if not already completed)
	deps := GetDependencies(taskJob.DependsOn)
	for _, depName := range deps {
		if execCtx.IsJobCompleted(depName) {
			continue
		}

		if _, depExists := allJobs[depName]; !depExists {
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusFailed)
			}
			return fmt.Errorf("dependency %q not found for task %q", depName, taskName)
		}

		// Create a synthetic step to execute the dependency as a task
		depStep := &model.Step{Task: depName}
		if err := e.executeTaskStep(ctx, execCtx, depStep, stepNode); err != nil {
			return err
		}
	}

	// Get the existing tree node for this task
	taskJobNode := execCtx.JobNodes[taskName]
	if taskJobNode == nil {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("task %q node not found in tree", taskName)
	}

	taskJobNode.Summarize = taskJob.Summarize
	stepNode.Summarize = step.Summarize

	// Add task node as child of step node so it appears expanded in the tree
	if stepNode != nil && taskJobNode != nil {
		stepNode.AddChild(taskJobNode.Node)
	}

	// Check if this step has a for loop
	if step.For != "" {
		// Handle task invocation with for loop
		return e.executeTaskStepWithLoop(ctx, execCtx, step, stepNode, taskJob, taskJobNode)
	}

	// Mark the task as running
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusRunning)
	}

	// Mark the task node itself as running
	taskJobNode.SetStatus(treeview.StatusRunning)
	execCtx.Render()

	// Capture task start time for logging
	var taskStartOffset float64
	if execCtx.EventLogger != nil {
		taskStartOffset = execCtx.EventLogger.GetElapsed()
	}
	taskJobNode.Node.SetStartOffset(taskStartOffset)
	taskStartTime := time.Now()

	// Create a new execution context for the task using the task's existing tree node
	taskCtx := execCtx.Copy()
	taskCtx.Depth++
	taskCtx.Job = taskJob
	taskCtx.CurrentJob = taskJobNode
	taskCtx.Context = ctx
	taskCtx.StepSequence = 0 // Reset step counter for new job

	err := func() error {
		if err := MergeVariables(taskJob.Decl, taskCtx); err != nil {
			return err
		}
		if err := ValidateJobRequirements(taskJob, taskCtx); err != nil {
			return err
		}
		if err := e.executeSteps(ctx, taskCtx, taskJob.Steps); err != nil {
			return err
		}
		return nil
	}()

	// Calculate task duration and log
	taskDuration := time.Since(taskStartTime)
	taskJobNode.Node.SetDuration(taskDuration.Seconds())

	taskID := "jobs." + taskName
	if execCtx.EventLogger != nil {
		result := eventlog.ResultPass
		if err != nil {
			result = eventlog.ResultFail
		}
		execCtx.EventLogger.LogExec(result, taskID, taskName, taskStartOffset, taskDuration.Milliseconds(), err)
	}

	if err != nil {
		taskJobNode.SetStatus(treeview.StatusFailed)
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		execCtx.MarkJobCompleted(taskName)
		return err
	}

	// Mark task and step as passed
	taskJobNode.SetStatus(treeview.StatusPassed)
	execCtx.MarkJobCompleted(taskName)
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusPassed)
	}
	return nil
}

// executeTaskStepWithLoop executes a task multiple times via a for loop with loop variables
func (e *Executor) executeTaskStepWithLoop(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, taskJob *model.Job, taskJobNode *treeview.TreeNode) error {
	defer execCtx.Render()

	// Expand the for loop to get iteration contexts
	exec := NewExecWithEnv(execCtx.Env)
	iterations, err := ExpandFor(execCtx, exec.ExecuteCommand)
	if err != nil {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return fmt.Errorf("failed to expand for loop: %w", err)
	}

	if len(iterations) == 0 {
		// No iterations, mark as passed
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusPassed)
		}
		return nil
	}

	// Execute task for each iteration
	var lastErr error
	for _, iter := range iterations {
		// Create execution context for this iteration with loop variables
		iterCtx := execCtx.Copy()
		for k, v := range iter.Variables {
			iterCtx.Variables[k] = v
		}
		iterCtx.Job = taskJob
		iterCtx.CurrentJob = taskJobNode
		iterCtx.Context = ctx

		if err := MergeVariables(taskJob.Decl, iterCtx); err != nil {
			taskJobNode.SetStatus(treeview.StatusFailed)
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusFailed)
			}
			return err
		}

		// Mark task as running
		taskJobNode.SetStatus(treeview.StatusRunning)

		// Validate job requirements (loop variables should satisfy requires)
		if err := ValidateJobRequirements(taskJob, iterCtx); err != nil {
			taskJobNode.SetStatus(treeview.StatusFailed)
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusFailed)
			}
			return err
		}

		// Execute the task job steps with iteration context
		if err := e.executeSteps(ctx, iterCtx, taskJob.Steps); err != nil {
			lastErr = err
			// Continue to next iteration even on error (collect all failures)
			// This matches yamlexpr behavior of processing all items
		}
	}

	// Update task node status based on results
	if lastErr != nil {
		taskJobNode.SetStatus(treeview.StatusFailed)
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		return lastErr
	}

	// Mark task and step as passed
	taskJobNode.SetStatus(treeview.StatusPassed)
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusPassed)
	}

	return nil
}

// interpolateVariables interpolates all string variables in a map using $(exec) and ${{ var }} syntax.
// Non-string values are passed through unchanged.
// Variables are evaluated in dependency order using topological sort.
// Returns the interpolated map or an error if interpolation fails.
func interpolateVariables(ctx *ExecutionContext, vars map[string]any) (map[string]any, error) {
	if vars == nil {
		return nil, nil
	}

	if ctx == nil {
		return vars, nil
	}

	// Build dependency graph
	deps := make(map[string][]string)
	for k, v := range vars {
		if strVal, ok := v.(string); ok {
			deps[k] = extractVariableDependencies(strVal, vars)
		} else {
			deps[k] = nil
		}
	}

	// Topological sort
	order, err := topologicalSort(deps)
	if err != nil {
		return nil, err
	}

	// Create a working context that accumulates resolved variables
	workCtx := &ExecutionContext{
		Variables: make(map[string]any),
		Env:       ctx.Env,
	}
	for k, v := range ctx.Variables {
		workCtx.Variables[k] = v
	}

	result := make(map[string]any)
	for _, k := range order {
		v := vars[k]
		if strVal, ok := v.(string); ok {
			interpolated, err := InterpolateString(strVal, workCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to interpolate variable %q: %w", k, err)
			}
			result[k] = interpolated
			workCtx.Variables[k] = interpolated
		} else {
			result[k] = v
			workCtx.Variables[k] = v
		}
	}
	return result, nil
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

// executeCmdsStep executes multiple commands as children of a step node
func (e *Executor) executeCmdsStep(ctx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node) error {
	children := stepNode.GetChildren()
	var lastErr error

	// Execute each command using its corresponding child node
	for i, cmd := range step.Cmds {
		var cmdNode *treeview.Node
		if i < len(children) {
			cmdNode = children[i]
		}

		// Mark command as running
		if cmdNode != nil {
			cmdNode.SetStatus(treeview.StatusRunning)
			execCtx.Render()
		}

		// Execute the command with the cmd node as the current step for output capture
		originalStep := execCtx.CurrentStep
		if cmdNode != nil {
			execCtx.CurrentStep = cmdNode
		}

		if err := e.executeCommand(ctx, execCtx, step, cmd); err != nil {
			if cmdNode != nil {
				cmdNode.SetStatus(treeview.StatusFailed)
			}
			execCtx.Render()
			lastErr = err
			// Continue to next command even on error to update all nodes
		} else {
			if cmdNode != nil {
				cmdNode.SetStatus(treeview.StatusPassed)
			}
			execCtx.Render()
		}

		// Restore original step
		execCtx.CurrentStep = originalStep
	}

	return lastErr
}

// IsEchoCommand checks if a command is a bare echo command.
func IsEchoCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	return strings.HasPrefix(trimmed, "echo ") && !strings.Contains(trimmed, "\n")
}

// evaluateEchoCommand executes an echo command and returns its output for use as a label
func evaluateEchoCommand(ctx context.Context, cmd string, env map[string]string) (string, error) {
	exec := NewExecWithEnv(env)
	output, err := exec.ExecuteCommandWithQuiet(cmd, false)
	if err != nil {
		return "", err
	}
	// Trim output and return
	return strings.TrimSpace(output), nil
}

// executeCommand runs a single command with interpolation and respects context timeout
func (e *Executor) executeCommand(ctx context.Context, execCtx *ExecutionContext, step *model.Step, cmd string) error {
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

	// Execute the command via bash with quiet mode, passing execution context env
	exec := NewExecWithEnv(execCtx.Env)

	// Determine if output should be captured for display with tree indentation
	// Check step passthru flag first, then job passthru flag
	shouldPassthru := step.Passthru || (execCtx.Job != nil && execCtx.Job.Passthru)

	// Determine TTY allocation: Job.TTY is authoritative, otherwise use Step.TTY
	useTTY := step.TTY || (execCtx.Job != nil && execCtx.Job.TTY)

	// If passthru is enabled, capture output to the node for display with tree indentation
	var writer *LineCapturingWriter
	if shouldPassthru && execCtx.CurrentStep != nil {
		writer = NewLineCapturingWriter()
		_, err = exec.ExecuteCommandWithWriter(writer, interpolated, useTTY)
	} else {
		_, err = exec.ExecuteCommandWithQuiet(interpolated, execCtx.Verbose)
	}

	if err != nil {
		// Return the error as-is if it's an ExecError, otherwise wrap it
		if execErr, ok := err.(ExecError); ok {
			return execErr
		}
		return fmt.Errorf("command execution %s failed: %w", execCtx.CurrentStep.ID, err)
	}

	// For echo commands, update the step node label with the output
	if IsEchoCommand(interpolated) && execCtx.CurrentStep != nil {
		output, err := evaluateEchoCommand(ctx, interpolated, execCtx.Env)
		if err == nil && output != "" {
			execCtx.CurrentStep.Name = output
		}
	}

	// Set output on node only after command completes successfully
	if writer != nil && execCtx.CurrentStep != nil {
		rawOutput := writer.String()
		lines, sanitizeErr := Sanitize(rawOutput)
		if sanitizeErr != nil {
			return fmt.Errorf("failed to sanitize output: %w", sanitizeErr)
		}
		if len(lines) > 0 {
			execCtx.CurrentStep.SetOutput(lines)
		}
	}

	return nil
}
