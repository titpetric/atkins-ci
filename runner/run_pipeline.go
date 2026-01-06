package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/treeview"
	"golang.org/x/sync/errgroup"
)

// RunPipelineWithLog runs a pipeline with optional logging to a file.
func RunPipelineWithLog(ctx context.Context, pipeline *model.Pipeline, job string, logFile string) error {
	return RunPipelineWithLogAndFile(ctx, pipeline, job, logFile, "")
}

// RunPipelineWithLogAndFile runs a pipeline with optional logging to a file and pipeline filename.
func RunPipelineWithLogAndFile(ctx context.Context, pipeline *model.Pipeline, job string, logFile string, pipelineFile string) error {
	logger, err := NewStepLoggerWithPipeline(logFile, pipelineFile)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	return RunPipelineWithLogger(ctx, pipeline, logger, job)
}

// RunPipelineWithLogger runs a pipeline with the given logger.
func RunPipelineWithLogger(ctx context.Context, pipeline *model.Pipeline, logger *StepLogger, job string) error {
	return runPipeline(ctx, pipeline, job, logger)
}

// RunPipeline runs a pipeline without logging.
func RunPipeline(ctx context.Context, pipeline *model.Pipeline, job string) error {
	return runPipeline(ctx, pipeline, job, nil)
}

func runPipeline(ctx context.Context, pipeline *model.Pipeline, job string, logger *StepLogger) error {
	tree := treeview.NewBuilder(pipeline.Name)
	root := tree.Root()

	display := treeview.NewDisplay()
	pipelineCtx := &ExecutionContext{
		Variables: make(map[string]interface{}),
		Env:       make(map[string]string),
		Results:   make(map[string]interface{}),
		Pipeline:  pipeline,
		Depth:     0,
		Builder:   tree,
		Display:   display,
		Context:   ctx,
		JobNodes:  make(map[string]*treeview.TreeNode),
		Logger:    logger,
	}

	// Copy environment variables
	for _, env := range os.Environ() {
		k, v := parseEnv(env)
		if k != "" {
			pipelineCtx.Env[k] = v
		}
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
	for jobName := range jobsToCreate {
		job := allJobs[jobName]
		jobLabel := jobName
		if job.Desc != "" {
			jobLabel = jobName + " - " + job.Desc
		}

		// Get job dependencies
		deps := GetDependencies(job.DependsOn)

		jobNode := tree.AddJobWithoutSteps(deps, jobLabel, job.Nested)

		// Populate children
		for _, step := range job.Steps {
			// Get the step command/name
			stepName := step.String()
			stepNode := &treeview.Node{
				Name:     stepName,
				Status:   treeview.StatusPending,
				Children: make([]*treeview.Node, 0),
				Deferred: step.IsDeferred(),
			}
			jobNode.AddChild(stepNode)
		}

		jobNodes[jobName] = jobNode
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

		jobCtx := *pipelineCtx
		jobCtx.Job = job
		jobCtx.Depth = 1

		// Get pre-created job node and mark it as running
		jobNode := jobNodes[jobName]
		jobNode.SetStatus(treeview.StatusRunning)
		jobCtx.CurrentJob = jobNode

		display.Render(root)

		if err := executor.ExecuteJob(ctx, job, &jobCtx, jobName); err != nil {
			jobMutex.Lock()
			jobCompleted[jobName] = true
			jobMutex.Unlock()
			return err
		}

		// Mark job as passed
		jobNode.SetStatus(treeview.StatusPassed)
		display.Render(root)

		// Store results
		jobMutex.Lock()
		jobCompleted[jobName] = true
		jobResults[jobName] = &jobCtx
		jobMutex.Unlock()

		return nil
	}

	eg := new(errgroup.Group)
	detached := 0
	count := 0

	for _, name := range jobOrder {
		job := allJobs[name]

		if job.Detach {
			detached++
			count++
			eg.Go(func() error {
				return executeJobWithDeps(name, job)
			})
			continue
		}

		if err := executeJobWithDeps(name, job); err != nil {
			root.SetStatus(treeview.StatusFailed)
			display.Render(root)
			return err
		}
		count++
	}

	// Wait for all detached jobs
	if detached > 0 {
		if err := eg.Wait(); err != nil {
			// Mark pipeline as failed
			root.SetStatus(treeview.StatusFailed)
			display.Render(root)

			return err
		}
	}

	// Mark pipeline as passed and render final tree
	root.SetStatus(treeview.StatusPassed)
	display.Render(root)

	//	fmt.Print(colors.BrightGreen(fmt.Sprintf("✓ PASS (%d jobs passing)\n", count)))
	return nil
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
