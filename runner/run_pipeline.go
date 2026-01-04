package runner

import (
	"context"
	"errors"
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

func RunPipeline(ctx context.Context, pipeline *model.Pipeline, job string) error {
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

	// Pre-populate all jobs as pending
	jobNodes := make(map[string]*treeview.TreeNode)
	for _, jobName := range jobOrder {
		job := allJobs[jobName]
		jobLabel := jobName
		if job.Desc != "" {
			jobLabel = jobName + " - " + job.Desc
		}

		// Get job dependencies
		deps := GetDependencies(job.DependsOn)

		jobNode := tree.AddJob(jobLabel, job, deps)

		// Populate children
		for _, step := range job.Steps {
			var stepNode *treeview.Node
			if step.Deferred {
				stepNode = &treeview.Node{
					Name:      step.Name,
					Status:    treeview.StatusPending,
					UpdatedAt: time.Now(),
					Children:  make([]*treeview.Node, 0),
					Deferred:  true,
				}
			} else {
				stepNode = &treeview.Node{
					Name:      step.Name,
					Status:    treeview.StatusPending,
					UpdatedAt: time.Now(),
					Children:  make([]*treeview.Node, 0),
				}
			}
			jobNode.AddChild(stepNode)
		}

		jobNodes[jobName] = jobNode
	}
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
				time.Sleep(100 * time.Millisecond)
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

		if err := executor.ExecuteJob(ctx, &jobCtx, jobName, job); err != nil {
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

		if job.Nested {
			continue
		}

		if job.Detach {
			detached++
			count++
			eg.Go(func() error {
				return executeJobWithDeps(name, job)
			})
			continue
		}

		if err := executeJobWithDeps(name, job); err != nil {
			root.Status = treeview.StatusFailed
			root.UpdatedAt = time.Now()

			display.Render(root)

			fmt.Println(colors.BrightRed("✗ FAIL"))

			var errorLog ExecError
			if errors.As(err, &errorLog) {
				if errorLog.Len() > 0 {
					fmt.Println(colors.BrightRed("Error: " + errorLog.Message))
					fmt.Print(errorLog.ErrorLog)
				}
			}
			return err
		}
		count++
	}

	// Wait for all detached jobs
	if detached > 0 {
		if err := eg.Wait(); err != nil {
			// Mark pipeline as failed
			root.Status = treeview.StatusFailed
			root.UpdatedAt = time.Now()
			display.Render(root)

			fmt.Println(colors.BrightRed("✗ FAIL"))
			// Print stderr if there's any error output
			var errorLog ExecError
			if errors.As(err, &errorLog) {
				if errorLog.Len() > 0 {
					fmt.Println(colors.BrightRed("Error: " + errorLog.Message))
					fmt.Print(errorLog.ErrorLog)
				}
			}
			return err
		}
	}

	// Mark pipeline as passed and render final tree
	root.Status = treeview.StatusPassed
	root.UpdatedAt = time.Now()
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
