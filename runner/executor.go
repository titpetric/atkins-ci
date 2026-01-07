package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/titpetric/atkins/model"
	"github.com/titpetric/atkins/treeview"
)

// LineCapturingWriter captures all output written to it
type LineCapturingWriter struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

// NewLineCapturingWriter creates a new LineCapturingWriter
func NewLineCapturingWriter() *LineCapturingWriter {
	return &LineCapturingWriter{}
}

// Write implements io.Writer
func (w *LineCapturingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

// GetLines returns all captured output as lines
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
func (e *Executor) ExecuteJob(parentCtx context.Context, job *model.Job, ctx *ExecutionContext, jobName string) error {
	// Ensure job.Name is set
	if job.Name == "" {
		job.Name = jobName
	}

	// Parse job timeout
	jobTimeout := parseTimeout(job.Timeout, e.opts.DefaultTimeout)

	// Create a child context with the job timeout
	jobCtx, cancel := context.WithTimeout(parentCtx, jobTimeout)
	defer cancel()

	// Store context in execution context for use in steps
	ctx.Context = jobCtx

	// Merge job variables into context with interpolation
	if job.Vars != nil {
		interpolated, err := interpolateVariables(ctx, job.Vars)
		if err != nil {
			return err
		}
		for k, v := range interpolated {
			ctx.Variables[k] = v
		}
	}

	// Merge job environment with interpolation
	if err := MergeEnv(job.Env, ctx); err != nil {
		return err
	}

	// Execute steps
	if len(job.Steps) > 0 {
		return e.executeSteps(jobCtx, ctx, job.Steps)
	}

	// Execute legacy cmd/cmds format
	emptyStep := &model.Step{}
	if job.Run != "" {
		return e.executeCommand(jobCtx, ctx, emptyStep, job.Run)
	}

	if job.Cmd != "" {
		return e.executeCommand(jobCtx, ctx, emptyStep, job.Cmd)
	}

	if len(job.Cmds) > 0 {
		for _, cmd := range job.Cmds {
			if err := e.executeCommand(jobCtx, ctx, emptyStep, cmd); err != nil {
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
				return e.executeStep(jobCtx, execCtx, steps[idx], idx)
			})
			continue
		}

		if err := wait(); err != nil {
			return err
		}

		if err := e.executeStep(jobCtx, execCtx, steps[idx], idx); err != nil {
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
			if err := e.executeStepWithNode(jobCtx, execCtx, step, stepNode); err != nil {
				return err
			}
		} else {
			// Fallback to executeStep if node not found
			if err := e.executeStep(jobCtx, execCtx, step, stepIdx); err != nil {
				return err
			}
		}
	}

	return nil
}

// executeStepWithNode runs a single step with a provided node
func (e *Executor) executeStepWithNode(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node) error {
	// Handle step-level environment variables
	stepCtx := execCtx.Copy()
	stepCtx.Context = jobCtx
	stepCtx.Variables = execCtx.Variables
	stepCtx.Step = step

	env := make(map[string]string)
	// Copy parent env
	for k, v := range execCtx.Env {
		env[k] = v
	}
	stepCtx.Env = env

	// Merge step-level env with interpolation
	if err := MergeEnv(step.Env, stepCtx); err != nil {
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
		if execCtx.Logger != nil {
			execCtx.Logger.LogSkip(jobName, seqIndex, stepName)
		}
		return nil
	}

	// Handle for loop expansion
	if step.For != "" {
		return e.executeStepWithForLoop(jobCtx, stepCtx, step, 0, stepNode)
	} else {
		// Handle task invocation
		if step.Task != "" {
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusRunning)
			}
			return e.executeTaskStep(jobCtx, stepCtx, step, stepNode)
		}
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
	return e.executeStepIteration(jobCtx, stepCtx, step, stepNode, cmd, 0)
}

// executeStep runs a single step
func (e *Executor) executeStep(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepIndex int) error {
	defer execCtx.Render()

	// Handle step-level environment variables
	stepCtx := execCtx.Copy()
	stepCtx.Context = jobCtx
	stepCtx.Variables = execCtx.Variables
	stepCtx.Step = step

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
	if err := MergeEnv(step.Env, stepCtx); err != nil {
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
		if execCtx.Logger != nil {
			execCtx.Logger.LogSkip(jobName, seqIndex, stepName)
		}
		return nil
	}

	// Handle task invocation
	if step.Task != "" {
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusRunning)
		}
		return e.executeTaskStep(jobCtx, stepCtx, step, stepNode)
	}

	// Handle for loop expansion
	if step.For != "" {
		stepNode.Summarize = step.Summarize
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusRunning)
		}
		if err := e.executeStepWithForLoop(jobCtx, stepCtx, step, stepIndex, stepNode); err != nil {
			stepNode.SetStatus(treeview.StatusFailed)
			return err
		}
		return nil
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
	return e.executeStepIteration(jobCtx, stepCtx, step, stepNode, cmd, stepIndex)
}

// executeStepWithForLoop handles for loop expansion and execution
// Each iteration becomes a separate execution with iteration variables overlaid on context
func (e *Executor) executeStepWithForLoop(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepIndex int, stepNode *treeview.Node) error {
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
			if step.Run != "" {
				cmdTemplate = step.Run
			} else if step.Cmd != "" {
				cmdTemplate = step.Cmd
			} else if len(step.Cmds) > 0 {
				cmdTemplate = strings.Join(step.Cmds, " && ")
			}
		}

		// Create node for each iteration with interpolated command
		for idx, iteration := range iterations {
			// Interpolate command with iteration variables
			iterCtx := execCtx.Copy()
			iterCtx.Variables = copyVariables(execCtx.Variables)

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

			// Add as child of the step node
			stepNode.AddChild(iterNode)
			iterationNodes = append(iterationNodes, iterNode)
		}
	}

	// Render tree with expanded iterations
	execCtx.Render()

	// Execute each iteration
	var lastErr error
	for idx, iteration := range iterations {
		// Create iteration context by overlaying iteration variables on parent context
		iterCtx := execCtx.Copy()
		iterCtx.Variables = copyVariables(execCtx.Variables)
		iterCtx.Context = jobCtx

		// Overlay iteration variables (they override parent variables)
		for k, v := range iteration.Variables {
			iterCtx.Variables[k] = v
		}

		// Merge step-level env with interpolation
		// This needs to happen before building the command so env vars can be interpolated
		if err := MergeEnv(step.Env, iterCtx); err != nil {
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusFailed)
			}
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
			if err := e.executeTaskStep(jobCtx, iterCtx, step, iterNode); err != nil {
				lastErr = err
				// Continue to next iteration even on error (collect all failures)
				// This matches yamlexpr behavior of processing all items
			}
		} else {
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

			// Execute this iteration with the iteration sub-node
			if err := e.executeStepIteration(jobCtx, iterCtx, step, iterNode, cmd, stepIndex); err != nil {
				lastErr = err
				// Continue to next iteration even on error (collect all failures)
				// This matches yamlexpr behavior of processing all items
			}
		}
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
func (e *Executor) executeStepIteration(jobCtx context.Context, stepCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, cmd string, stepIndex int) error {
	// Get step name for logging
	stepName := step.Name
	if stepName == "" && stepNode != nil {
		stepName = stepNode.Name
	}

	// Get the next sequential step index for this job
	// Note: We use the original stepCtx to increment the counter
	// This ensures that all steps/iterations in a job get unique sequential indices
	seqIndex := stepCtx.NextStepIndex()

	// Log RUN event
	jobName := ""
	if stepCtx.Job != nil {
		jobName = stepCtx.Job.Name
	}
	if stepCtx.Logger != nil {
		stepCtx.Logger.LogRun(jobName, seqIndex, stepName)
	}

	// Track start time for duration
	startTime := time.Now()

	// Mark step as running and render immediately to show state transition
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusRunning)
		stepCtx.Render()
	}

	// Handle cmds: if step has multiple commands and child nodes exist, execute each command individually
	var err error
	if len(step.Cmds) > 0 && stepNode != nil && stepNode.HasChildren() {
		err = e.executeCmdsStep(jobCtx, stepCtx, step, stepNode)
	} else {
		err = e.executeCommand(jobCtx, stepCtx, step, cmd)
	}

	// Calculate duration in milliseconds
	duration := time.Since(startTime).Milliseconds()

	// Update tree node status and log result
	if stepNode != nil {
		if err != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		} else {
			stepNode.SetStatus(treeview.StatusPassed)
		}
	}

	if err != nil {
		if stepCtx.Logger != nil {
			stepCtx.Logger.LogFail(jobName, seqIndex, stepName, err, duration)
		}
		return err
	}
	if stepCtx.Logger != nil {
		stepCtx.Logger.LogPass(jobName, seqIndex, stepName, duration)
	}

	stepCtx.Render()
	return nil
}

// executeTaskStep executes a task/job from within a step
// Supports both simple task invocation and for loop task invocation with loop variables
func (e *Executor) executeTaskStep(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node) error {
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

	// If the task is nested and the step node exists, add task node as child of step node
	// so it appears in the tree under the step
	if taskJob.Nested && stepNode != nil && taskJobNode != nil {
		stepNode.AddChild(taskJobNode.Node)
	}

	// Check if this step has a for loop
	if step.For != "" {
		// Handle task invocation with for loop
		return e.executeTaskStepWithLoop(jobCtx, execCtx, step, stepNode, taskJob, taskJobNode)
	}

	// Mark the task as running
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusRunning)
	}

	// Mark the task node itself as running
	taskJobNode.SetStatus(treeview.StatusRunning)
	execCtx.Render()

	// Create a new execution context for the task using the task's existing tree node
	taskCtx := execCtx.Copy()
	taskCtx.Variables = copyVariables(execCtx.Variables)
	taskCtx.Depth++
	taskCtx.Job = taskJob
	taskCtx.CurrentJob = taskJobNode
	taskCtx.Context = jobCtx

	// Interpolate task-level variables (supports $(exec) syntax)
	if taskJob.Vars != nil {
		interpolated, err := interpolateVariables(taskCtx, taskJob.Vars)
		if err != nil {
			taskJobNode.SetStatus(treeview.StatusFailed)
			if stepNode != nil {
				stepNode.SetStatus(treeview.StatusFailed)
			}
			execCtx.Render()
			return err
		}
		for k, v := range interpolated {
			taskCtx.Variables[k] = v
		}
	}

	// Validate job requirements
	if err := ValidateJobRequirements(taskJob, taskCtx); err != nil {
		taskJobNode.SetStatus(treeview.StatusFailed)
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		execCtx.Render()
		return err
	}

	// Execute the task job steps
	if err := e.executeSteps(jobCtx, taskCtx, taskJob.Steps); err != nil {
		taskJobNode.SetStatus(treeview.StatusFailed)
		if stepNode != nil {
			stepNode.SetStatus(treeview.StatusFailed)
		}
		execCtx.Render()
		return err
	}

	// Mark task and step as passed
	taskJobNode.SetStatus(treeview.StatusPassed)
	if stepNode != nil {
		stepNode.SetStatus(treeview.StatusPassed)
	}
	execCtx.Render()

	return nil
}

// executeTaskStepWithLoop executes a task multiple times via a for loop with loop variables
func (e *Executor) executeTaskStepWithLoop(jobCtx context.Context, execCtx *ExecutionContext, step *model.Step, stepNode *treeview.Node, taskJob *model.Job, taskJobNode *treeview.TreeNode) error {
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
		// Merge iteration variables (loop variables) with parent variables
		// Iteration variables take precedence over parent variables
		for k, v := range iter.Variables {
			iterCtx.Variables[k] = v
		}
		iterCtx.Job = taskJob
		iterCtx.CurrentJob = taskJobNode
		iterCtx.Context = jobCtx

		// Interpolate task-level variables (supports $(exec) syntax)
		if taskJob.Vars != nil {
			interpolated, err := interpolateVariables(iterCtx, taskJob.Vars)
			if err != nil {
				taskJobNode.SetStatus(treeview.StatusFailed)
				if stepNode != nil {
					stepNode.SetStatus(treeview.StatusFailed)
				}
				return err
			}
			for k, v := range interpolated {
				iterCtx.Variables[k] = v
			}
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
		if err := e.executeSteps(jobCtx, iterCtx, taskJob.Steps); err != nil {
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

// copyVariables creates a shallow copy of a variables map
func copyVariables(vars map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for k, v := range vars {
		copy[k] = v
	}
	return copy
}

// interpolateVariables interpolates all string variables in a map using $(exec) and ${{ var }} syntax.
// Non-string values are passed through unchanged.
// Returns the interpolated map or an error if interpolation fails.
func interpolateVariables(ctx *ExecutionContext, vars map[string]interface{}) (map[string]interface{}, error) {
	if vars == nil {
		return nil, nil
	}

	result := make(map[string]interface{})
	for k, v := range vars {
		// Only interpolate string values
		if strVal, ok := v.(string); ok {
			interpolated, err := InterpolateString(strVal, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to interpolate variable %q: %w", k, err)
			}
			result[k] = interpolated
		} else {
			result[k] = v
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

		// Execute the command
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
	}

	return lastErr
}

// isEchoCommand checks if a command is a bare echo command
func isEchoCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	return strings.HasPrefix(trimmed, "echo ")
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

	// If passthru is enabled, capture output to the node for display with tree indentation
	var writer *LineCapturingWriter
	if shouldPassthru && execCtx.CurrentStep != nil {
		writer = NewLineCapturingWriter()
		_, err = exec.ExecuteCommandWithWriter(interpolated, writer)
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

	// Set output on node only after command completes successfully
	if writer != nil && execCtx.CurrentStep != nil {
		lines := writer.GetLines()
		if len(lines) > 0 {
			execCtx.CurrentStep.SetOutput(lines)
		}
	}

	return nil
}
