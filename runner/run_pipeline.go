package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/titpetric/atkins/colors"
	"github.com/titpetric/atkins/eventlog"
	"github.com/titpetric/atkins/model"
	"github.com/titpetric/atkins/treeview"
)

// PipelineOptions contains options for running a pipeline.
type PipelineOptions struct {
	Job          string
	LogFile      string
	PipelineFile string
	Debug        bool
	FinalOnly    bool
}

// RunPipeline runs a pipeline with the given options.
func RunPipeline(ctx context.Context, pipeline *model.Pipeline, opts PipelineOptions) error {
	var logger *eventlog.Logger
	if opts.LogFile != "" || opts.PipelineFile != "" {
		logger = eventlog.NewLogger(opts.LogFile, pipeline.Name, opts.PipelineFile, opts.Debug)
	}
	return runPipeline(ctx, pipeline, opts.Job, logger, opts.FinalOnly)
}

func runPipeline(ctx context.Context, pipeline *model.Pipeline, job string, logger *eventlog.Logger, finalOnly bool) error {
	tree := treeview.NewBuilder(pipeline.Name)
	root := tree.Root()

	display := treeview.NewDisplayWithFinal(finalOnly)
	pipelineCtx := &ExecutionContext{
		Variables:   make(map[string]any),
		Env:         make(map[string]string),
		Results:     make(map[string]any),
		Pipeline:    pipeline,
		Depth:       0,
		Builder:     tree,
		Display:     display,
		Context:     ctx,
		JobNodes:    make(map[string]*treeview.TreeNode),
		EventLogger: logger,
	}

	// Copy environment variables from OS
	for _, env := range os.Environ() {
		k, v := parseEnv(env)
		if k != "" {
			pipelineCtx.Env[k] = v
		}
	}

	if err := MergeVariables(pipeline.Decl, pipelineCtx); err != nil {
		return err
	}

	// Resolve jobs to run
	allJobs := pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = pipeline.Tasks
	}

	jobOrder, err := ResolveJobDependencies(allJobs, job)
	if err != nil {
		fmt.Printf("%s %s\n", colors.BrightRed("ERROR:"), err)
		os.Exit(1)
	}

	// Pre-populate all jobs as pending - include all jobs that might be invoked
	jobNodes := make(map[string]*treeview.TreeNode)
	jobsToCreate := make(map[string]bool)

	// Recursively find all jobs that might be invoked
	var findInvokedJobs func(jobName string)
	findInvokedJobs = func(jobName string) {
		if jobsToCreate[jobName] {
			return // Already processed
		}
		jobsToCreate[jobName] = true

		job, exists := allJobs[jobName]
		if !exists {
			return
		}

		// Recursively find all task references
		for _, step := range job.Steps {
			if step.Task != "" {
				findInvokedJobs(step.Task)
			}
		}
	}

	// Start with jobs in order
	for _, jobName := range jobOrder {
		findInvokedJobs(jobName)
	}

	// Create job nodes for all jobs that might be invoked
	// Only add root-level jobs to the tree display; nested jobs are added when invoked as tasks
	jobsToCreateSorted := treeview.SortByOrder(jobsToCreate, jobOrder)
	for _, jobName := range jobsToCreateSorted {
		job := allJobs[jobName]
		jobLabel := jobName
		if job.Desc != "" {
			jobLabel = jobName + " - " + job.Desc
		}

		// Get job dependencies
		deps := GetDependencies(job.DependsOn)

		// Check if this job is in the root execution order
		isRootJob := false
		for _, rootName := range jobOrder {
			if rootName == jobName {
				isRootJob = true
				break
			}
		}

		// Only add to tree if it's in jobOrder (root-level execution)
		if isRootJob {
			jobNode := tree.AddJobWithoutSteps(deps, jobLabel, job.Nested)
			jobNode.Summarize = job.Summarize

			// Populate children
			for _, step := range job.Steps {
				stepNode := treeview.NewPendingStepNode(step.String(), step.IsDeferred(), step.Summarize)

				// If step has multiple commands, create child nodes for each command
				if len(step.Cmds) > 0 {
					for _, cmd := range step.Cmds {
						stepNode.AddChild(treeview.NewCmdNode(cmd))
					}
				}

				jobNode.AddChild(stepNode)
			}

			jobNodes[jobName] = jobNode
		} else {
			// For non-root jobs (only invoked as tasks), create nodes but don't add to tree
			jobNode := treeview.NewNode(jobLabel)
			jobNode.Summarize = job.Summarize

			// Populate children
			for _, step := range job.Steps {
				stepNode := treeview.NewPendingStepNode(step.String(), step.IsDeferred(), step.Summarize)

				// If step has multiple commands, create child nodes for each command
				if len(step.Cmds) > 0 {
					for _, cmd := range step.Cmds {
						stepNode.AddChild(treeview.NewCmdNode(cmd))
					}
				}

				jobNode.AddChild(stepNode)
			}

			jobNodes[jobName] = &treeview.TreeNode{Node: jobNode}
		}
	}
	pipelineCtx.JobNodes = jobNodes
	display.Render(root)

	executor := NewExecutor()

	// Track job completion status
	jobCompleted := make(map[string]bool)
	jobResults := make(map[string]*ExecutionContext)
	var jobMutex sync.Mutex

	// Helper to execute a job (with dependency checking)
	executeJobWithDeps := func(jobName string, job *model.Job) error {
		// Wait for dependencies if any
		deps := GetDependencies(job.DependsOn)
		for _, dep := range deps {
			for {
				jobMutex.Lock()
				if jobCompleted[dep] {
					jobMutex.Unlock()
					break
				}
				jobMutex.Unlock()
				time.Sleep(50 * time.Millisecond)
			}
		}

		jobCtx := pipelineCtx.Copy()
		jobCtx.Job = job
		jobCtx.Depth = 1
		jobCtx.StepSequence = 0 // Reset step counter for each job

		// Get pre-created job node and mark it as running
		jobNode := jobNodes[jobName]
		jobNode.SetStatus(treeview.StatusRunning)
		jobCtx.CurrentJob = jobNode

		// Capture job start time
		var jobStartOffset float64
		if logger != nil {
			jobStartOffset = logger.GetElapsed()
		}
		jobNode.Node.SetStartOffset(jobStartOffset)
		jobStartTime := time.Now()

		display.Render(root)

		execErr := executor.ExecuteJob(ctx, jobCtx)

		// Calculate job duration
		jobDuration := time.Since(jobStartTime)
		jobNode.Node.SetDuration(jobDuration.Seconds())

		// Log job event
		jobID := "jobs." + jobName
		if logger != nil {
			result := eventlog.ResultPass
			if execErr != nil {
				result = eventlog.ResultFail
			}
			logger.LogExec(jobID, jobName, result, jobStartOffset, jobDuration.Milliseconds(), execErr)
		}

		if execErr != nil {
			jobMutex.Lock()
			jobCompleted[jobName] = true
			jobMutex.Unlock()
			return execErr
		}

		// Mark job as passed
		jobNode.SetStatus(treeview.StatusPassed)
		display.Render(root)

		// Store results
		jobMutex.Lock()
		jobCompleted[jobName] = true
		jobResults[jobName] = jobCtx
		jobMutex.Unlock()

		return nil
	}

	eg := new(errgroup.Group)
	detached := 0
	count := 0

	for _, name := range jobOrder {
		job := allJobs[name]

		if job == nil {
			return fmt.Errorf("job %q not found in pipeline", name)
		}

		if job.Detach {
			detached++
			count++
			// Capture job and name by value to avoid closure variable capture issues
			jobCopy := job
			nameCopy := name
			eg.Go(func() error {
				return executeJobWithDeps(nameCopy, jobCopy)
			})
			continue
		}

		if err := executeJobWithDeps(name, job); err != nil {
			root.SetStatus(treeview.StatusFailed)
			display.Render(root)

			// If not a TTY, print final tree at the end
			if !display.IsTerminal() {
				display.RenderStatic(root)
			}

			// Write event log on failure
			writeEventLog(logger, root, err)

			return err
		}
		count++
	}

	// Wait for all detached jobs
	var runErr error
	if detached > 0 {
		if err := eg.Wait(); err != nil {
			// Mark pipeline as failed
			root.SetStatus(treeview.StatusFailed)
			display.Render(root)
			runErr = err
		}
	}

	if runErr == nil {
		// Mark pipeline as passed and render final tree
		root.SetStatus(treeview.StatusPassed)
	}
	display.Render(root)

	// If not a TTY, print final tree at the end
	if !display.IsTerminal() {
		display.RenderStatic(root)
	}

	// Write event log
	writeEventLog(logger, root, runErr)

	return runErr
}

// writeEventLog writes the final event log to the file.
func writeEventLog(logger *eventlog.Logger, root *treeview.Node, runErr error) {
	if logger == nil {
		return
	}

	// Set root duration
	root.SetDuration(logger.GetElapsed())

	// Convert tree to state
	state := eventlog.NodeToStateNode(root)

	// Count steps and build summary
	total, passed, failed, skipped := eventlog.CountSteps(state)

	result := eventlog.ResultPass
	if runErr != nil || failed > 0 {
		result = eventlog.ResultFail
	}

	stats := eventlog.CaptureRuntimeStats()
	summary := &eventlog.RunSummary{
		Duration:     logger.GetElapsed(),
		TotalSteps:   total,
		PassedSteps:  passed,
		FailedSteps:  failed,
		SkippedSteps: skipped,
		Result:       result,
		MemoryAlloc:  stats.MemoryAlloc,
		Goroutines:   stats.Goroutines,
	}

	logger.Write(state, summary)
}

func indent(depth int) string {
	return strings.Repeat("  ", depth)
}

func parseEnv(env string) (string, string) {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return env[:i], env[i+1:]
		}
	}
	return "", ""
}
