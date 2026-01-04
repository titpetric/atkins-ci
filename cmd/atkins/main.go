package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/runner"
	"golang.org/x/sync/errgroup"
)

func fatalf(message string, args ...any) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func fatal(message string) {
	fatalf("%s", message)
}

func main() {
	var pipelineFile string
	var job string
	var listFlag bool
	var lintFlag bool

	flag.StringVar(&pipelineFile, "file", "atkins.yml", "Path to pipeline file")
	flag.StringVar(&job, "job", "", "Specific job to run (optional)")
	flag.BoolVar(&listFlag, "l", false, "List pipeline jobs and dependencies")
	flag.BoolVar(&lintFlag, "lint", false, "Lint pipeline for errors")
	flag.Parse()

	// Handle positional argument as job name
	args := flag.Args()
	if len(args) > 0 {
		if args[0] == "lint" {
			lintFlag = true
		} else {
			job = args[0]
		}
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(pipelineFile)
	if err != nil {
		fatalf("%s %v\n", colors.BrightRed("ERROR:"), err)
	}

	// Load and parse pipeline
	pipelines, err := runner.LoadPipeline(absPath)
	if err != nil {
		fatalf("%s %s\n", colors.BrightRed("ERROR:"), err)
	}

	if len(pipelines) == 0 {
		fatalf("%s No pipelines found\n", colors.BrightRed("ERROR:"))
	}

	// Handle lint mode
	if lintFlag {
		for _, pipeline := range pipelines {
			linter := runner.NewLinter(pipeline)
			errors := linter.Lint()
			if len(errors) > 0 {
				fmt.Printf("%s Pipeline '%s' has errors:\n", colors.BrightRed("✗"), pipeline.Name)
				for _, lintErr := range errors {
					fmt.Printf("  %s: %s\n", lintErr.Job, lintErr.Detail)
				}
				os.Exit(1)
			}
		}
		fmt.Printf("%s Pipeline '%s' is valid\n", colors.BrightGreen("✓"), pipelines[0].Name)
		return
	}

	// Handle list mode
	if listFlag {
		for _, pipeline := range pipelines {
			linter := runner.NewLinter(pipeline)
			errors := linter.Lint()
			if len(errors) > 0 {
				fmt.Printf("%s Pipeline '%s' has dependency errors:\n", colors.BrightRed("✗"), pipeline.Name)
				for _, lintErr := range errors {
					fmt.Printf("  %s: %s\n", lintErr.Job, lintErr.Detail)
				}
				os.Exit(1)
			}
			runner.ListPipeline(pipeline)
		}
		return
	}

	// Run pipeline(s)
	var wg sync.WaitGroup
	wg.Add(len(pipelines))
	var exitCode int
	var failedPipeline string
	fmt.Printf("Found %d pipelines\n", len(pipelines))
	for _, pipeline := range pipelines {
		if err := runPipeline(&wg, pipeline, job); err != nil {
			exitCode = 1
			failedPipeline = pipeline.Name
		}
	}
	wg.Wait()

	// Print any captured error output with formatting
	runner.ErrorLogMutex.Lock()
	if runner.ErrorLog.Len() > 0 {
		fmt.Fprintf(os.Stderr, "\nAn error occurred in %q pipeline:\n\n", failedPipeline)
		fmt.Fprintf(os.Stderr, "  Exit code: %d\n", runner.LastExitCode)
		fmt.Fprintf(os.Stderr, "  Error output:\n")
		// Indent the error output
		for _, line := range strings.Split(runner.ErrorLog.String(), "\n") {
			if line != "" {
				fmt.Fprintf(os.Stderr, "    %s\n", line)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
	runner.ErrorLogMutex.Unlock()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runPipeline(wg *sync.WaitGroup, pipeline *model.Pipeline, job string) error {
	defer wg.Done()

	// Create a root context for the pipeline
	rootCtx := context.Background()

	// Create execution tree
	tree := runner.NewExecutionTree(pipeline.Name)

	// Create execution context
	// Create tree renderer for in-place updates
	renderer := runner.NewTreeRenderer()

	ctx := &model.ExecutionContext{
		Variables: make(map[string]interface{}),
		Env:       make(map[string]string),
		Results:   make(map[string]interface{}),
		QuietMode: 0,
		Pipeline:  pipeline.Name,
		Depth:     0,
		Tree:      tree,
		Renderer:  renderer,
		Context:   rootCtx,
	}

	// Copy environment variables
	for _, env := range os.Environ() {
		k, v := parseEnv(env)
		if k != "" {
			ctx.Env[k] = v
		}
	}

	// Initial tree render
	renderer.Render(tree)

	// Resolve jobs to run
	allJobs := pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = pipeline.Tasks
	}

	jobOrder, err := runner.ResolveJobDependencies(allJobs, job)
	if err != nil {
		fatalf("%s %s\n", colors.BrightRed("ERROR:"), err)
	}

	jobsToRun := make(map[string]*model.Job)
	for _, jobName := range jobOrder {
		jobsToRun[jobName] = allJobs[jobName]
	}

	// Pre-populate all jobs as pending
	jobNodes := make(map[string]*runner.TreeNode)
	for jobName, jobDef := range jobsToRun {
		jobLabel := jobName
		if jobDef.Desc != "" {
			jobLabel = jobName + " - " + jobDef.Desc
		}
		
		// Get job dependencies
		deps := runner.GetDependencies(jobDef.DependsOn)
		var jobNode *runner.TreeNode
		if len(deps) > 0 {
			jobNode = tree.AddJobWithDeps(jobLabel, deps)
		} else {
			jobNode = tree.AddJob(jobLabel)
		}

		// Populate children
		for _, step := range jobDef.Steps {
			var pendingNode *runner.TreeNode
			if step.Deferred {
				pendingNode = &runner.TreeNode{
					Name:      step.Name,
					Status:    runner.StatusPending,
					UpdatedAt: time.Now(),
					Children:  make([]*runner.TreeNode, 0),
					Deferred:  true,
				}
			} else {
				pendingNode = &runner.TreeNode{
					Name:      step.Name,
					Status:    runner.StatusPending,
					UpdatedAt: time.Now(),
					Children:  make([]*runner.TreeNode, 0),
				}
			}
			jobNode.Children = append(jobNode.Children, pendingNode)
		}

		jobNodes[jobName] = jobNode
	}
	renderer.Render(tree)

	executor := runner.NewExecutor()

	// Track job completion status
	jobCompleted := make(map[string]bool)
	jobResults := make(map[string]*model.ExecutionContext)
	var jobMutex sync.Mutex

	// Helper to execute a job (with dependency checking)
	executeJobWithDeps := func(jobName string, jobDef *model.Job) error {
		// Wait for dependencies if any
		deps := runner.GetDependencies(jobDef.DependsOn)
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

		jobCtx := *ctx
		jobCtx.Job = jobName
		jobCtx.JobDesc = jobDef.Desc
		jobCtx.Depth = 1

		// Get pre-created job node and mark it as running
		jobNode := jobNodes[jobName]
		jobNode.SetStatus(runner.StatusRunning)
		jobCtx.CurrentJob = jobNode
		renderer.Render(tree)

		if err := executor.ExecuteJob(rootCtx, &jobCtx, jobName, jobDef); err != nil {
			jobMutex.Lock()
			jobCompleted[jobName] = true
			jobMutex.Unlock()
			return err
		}

		// Mark job as passed
		jobNode.SetStatus(runner.StatusPassed)
		renderer.Render(tree)

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

	for name, job := range jobsToRun {
		if job.Detach {
			detached++
			count++
			eg.Go(func() error {
				return executeJobWithDeps(name, jobsToRun[name])
			})
			continue
		}

		if err := executeJobWithDeps(name, jobsToRun[name]); err != nil {
			// Mark pipeline as failed
			tree.Root.Status = runner.StatusFailed
			tree.Root.UpdatedAt = time.Now()
			// Render final tree
			renderer.Render(tree)
			fmt.Println(colors.BrightRed("✗ FAIL"))
			// Print stderr if there's any error output
			if ctx.QuietMode > 0 && runner.ErrorLog.Len() > 0 {
				fmt.Println(colors.BrightRed("Error output:"))
				fmt.Print(runner.ErrorLog.String())
			}
			return err
		}
		count++
	}

	// Wait for all detached jobs
	if detached > 0 {
		if err := eg.Wait(); err != nil {
			// Mark pipeline as failed
			tree.Root.Status = runner.StatusFailed
			tree.Root.UpdatedAt = time.Now()
			renderer.Render(tree)

			fmt.Println(colors.BrightRed("✗ FAIL"))
			// Print stderr if there's any error output
			if runner.ErrorLog.Len() > 0 {
				fmt.Println(colors.BrightRed("Error output:"))
				fmt.Print(runner.ErrorLog.String())
			}
			return err
		}
	}

	// Mark pipeline as passed and render final tree
	tree.Root.Status = runner.StatusPassed
	tree.Root.UpdatedAt = time.Now()
	renderer.Render(tree)

	fmt.Print(colors.BrightGreen(fmt.Sprintf("✓ PASS (%d jobs passing)\n", count)))
	return nil
}

func breadcrumb(ctx *model.ExecutionContext) string {
	parts := []string{ctx.Pipeline}
	if ctx.Job != "" {
		parts = append(parts, ctx.Job)
	}
	if ctx.Step != "" {
		parts = append(parts, ctx.Step)
	}
	return strings.Join(parts, " > ")
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
