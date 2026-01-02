package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/runner"
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

	flag.StringVar(&pipelineFile, "file", "atkins.yml", "Path to pipeline file")
	flag.StringVar(&job, "job", "default", "Specific job to run (optional, runs default if empty)")
	flag.Parse()

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

	colors.PrintInfo("Pipelines", fmt.Sprintf("%d", len(pipelines)))

	var wg sync.WaitGroup
	wg.Add(len(pipelines))
	for _, pipeline := range pipelines {
		fmt.Println(colors.BrightGreen("  ⚙️ PIPELINE COMPLETED SUCCESSFULLY"))
		if err := runPipeline(&wg, pipeline, job); err != nil {
			fmt.Println(colors.BrightRed("  ✖ PIPELINE FAILED"))
			fatalf("pipeline error: %v", err)
		}
		fmt.Println(colors.BrightGreen("  ✓ PIPELINE COMPLETED SUCCESSFULLY"))
	}
	wg.Wait()
}

func runPipeline(wg *sync.WaitGroup, pipeline *model.Pipeline, job string) error {
	defer wg.Done()

	colors.PrintHeader(fmt.Sprintf("atkins - %s", pipeline.Name))

	// Create execution context
	ctx := &model.ExecutionContext{
		Variables: make(map[string]interface{}),
		Env:       make(map[string]string),
		Results:   make(map[string]interface{}),
	}

	// Copy environment variables
	for _, env := range os.Environ() {
		k, v := parseEnv(env)
		if k != "" {
			ctx.Env[k] = v
		}
	}

	// Execute jobs
	var jobsToRun map[string]*model.Job
	if job != "" {
		j, ok := pipeline.Jobs[job]
		if !ok {
			fatalf("%s Job '%s' not found\n", colors.BrightRed("ERROR:"), job)
			os.Exit(1)
		}
		jobsToRun = map[string]*model.Job{job: j}
	} else {
		jobsToRun = pipeline.Jobs
		if len(jobsToRun) == 0 {
			jobsToRun = pipeline.Tasks
		}
	}

	colors.PrintInfo("Jobs", fmt.Sprintf("%d discovered", len(jobsToRun)))

	executor := runner.NewExecutor()
	jobCount := 0
	for jobName, jobDef := range jobsToRun {
		jobCount++
		colors.PrintSectionStart(fmt.Sprintf("JOB %d/%d: %s", jobCount, len(jobsToRun), jobName))
		if jobDef.Desc != "" {
			fmt.Printf("   %s\n", colors.Dim(jobDef.Desc))
		}

		if err := executor.ExecuteJob(ctx, jobName, jobDef); err != nil {
			colors.PrintSectionEnd(jobName, false)
			fmt.Fprintf(os.Stderr, "   %s\n", colors.BrightRed(err.Error()))
			return err
		} else {
			colors.PrintSectionEnd(jobName, true)
		}
	}
	return nil
}

func parseEnv(env string) (string, string) {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return env[:i], env[i+1:]
		}
	}
	return "", ""
}
