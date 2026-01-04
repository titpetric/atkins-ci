package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/spinner"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

// TreeRenderer manages in-place tree rendering with ANSI cursor control
type TreeRenderer struct {
	lastLineCount int
	mu            sync.Mutex
	isTerminal    bool
}

// NewTreeRenderer creates a new tree renderer
func NewTreeRenderer() *TreeRenderer {
	// Check if stdout is a TTY (interactive terminal)
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	return &TreeRenderer{
		lastLineCount: 0,
		isTerminal:    isTerminal,
	}
}

// Render outputs the tree, updating in-place if previously rendered
func (tr *TreeRenderer) Render(tree *ExecutionTree) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Only render if stdout is a TTY (interactive terminal)
	// This prevents duplicate output when piped or redirected
	if !tr.isTerminal {
		return
	}

	output := tree.RenderTree()
	lineCount := countOutputLines(output)

	if tr.lastLineCount > 0 {
		// Move cursor up, clear to end of display
		fmt.Printf("\033[%dA\033[J", tr.lastLineCount)
	}

	fmt.Print(output)
	tr.lastLineCount = lineCount
}

func indent(depth int) string {
	return strings.Repeat("  ", depth)
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
func (e *Executor) ExecuteJob(parentCtx context.Context, ctx *model.ExecutionContext, jobName string, job *model.Job) error {
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
func (e *Executor) executeSteps(jobCtx context.Context, execCtx *model.ExecutionContext, steps []model.Step) error {
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
func (e *Executor) executeStep(jobCtx context.Context, execCtx *model.ExecutionContext, step model.Step, stepIndex int) error {
	// Handle step-level environment variables
	stepCtx := &model.ExecutionContext{
		Variables: execCtx.Variables,
		Env:       make(map[string]string),
		Results:   execCtx.Results,
		QuietMode: execCtx.QuietMode,
		Pipeline:  execCtx.Pipeline,
		Job:       execCtx.Job,
		Step:      step.Name,
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

	// Get step node from tree
	var stepNode *TreeNode
	if jobNode, ok := execCtx.CurrentJob.(*TreeNode); ok {
		if stepIndex < len(jobNode.Children) {
			stepNode = jobNode.Children[stepIndex]
		}
	}

	// Mark step as running
	if stepNode != nil {
		stepNode.SetStatus(StatusRunning)
	}

	// Start spinner and execute command
	s := spinner.New()
	s.Start()

	// Channel to signal command completion
	cmdDone := make(chan error)
	go func() {
		// In TTY mode (interactive terminal), suppress output from detached steps
		// to prevent breaking the tree rendering. The output will be shown at the end if there's an error.
		var quietMode int
		if step.Detach {
			if renderer, ok := execCtx.Renderer.(*TreeRenderer); ok {
				if renderer.isTerminal {
					quietMode = 1 // suppress stdout for detached steps in TTY mode
				}
			}
		}

		// Execute command with appropriate quiet mode
		if quietMode > 0 {
			_, err := ExecuteCommandWithQuiet(cmd, quietMode)
			cmdDone <- err
		} else {
			cmdDone <- e.executeCommand(jobCtx, stepCtx, cmd)
		}
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
					stepNode.SetStatus(StatusFailed)
					return err
				}
				stepNode.SetStatus(StatusPassed)
				// Clear error log on successful step
				ErrorLogMutex.Lock()
				ErrorLog.Reset()
				ErrorLogMutex.Unlock()
			}

			// Increment step counters
			execCtx.StepsCount++
			execCtx.StepsPassed++

			// Render tree with final state
			if tree, ok := execCtx.Tree.(*ExecutionTree); ok {
				if renderer, ok := execCtx.Renderer.(*TreeRenderer); ok {
					renderer.Render(tree)
				}
			}
			return nil

		case <-tickerTicker.C:
			if stepNode != nil {
				stepNode.SetSpinner(s.String())
				// Render tree with updated spinner
				if tree, ok := execCtx.Tree.(*ExecutionTree); ok {
					if renderer, ok := execCtx.Renderer.(*TreeRenderer); ok {
						renderer.Render(tree)
					}
				}
			}
		}
	}
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
func (e *Executor) executeCommand(ctx context.Context, execCtx *model.ExecutionContext, cmd string) error {
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
