package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/runner"
	"gopkg.in/yaml.v2"
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
	var debug bool
	var logFile string

	flag.StringVar(&pipelineFile, "file", "atkins.yml", "Path to pipeline file")
	flag.StringVar(&job, "job", "", "Specific job to run")
	flag.BoolVar(&listFlag, "l", false, "List pipeline jobs and dependencies")
	flag.BoolVar(&lintFlag, "lint", false, "Lint pipeline for errors")
	flag.BoolVar(&debug, "debug", false, "Print debug data")
	flag.StringVar(&logFile, "log", "", "Log file path for command execution (e.g., atkins.log)")
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

			if debug {
				b, _ := yaml.Marshal(pipeline)
				fmt.Printf("%s\n", string(b))
			}

			if err := runner.ListPipeline(pipeline); err != nil {
				fmt.Printf("%s %s\n", "ERROR:", err)
				os.Exit(1)
			}
		}
		return
	}

	// Run pipeline(s)
	var exitCode int
	var failedPipeline string

	ctx := context.TODO()

	for _, pipeline := range pipelines {
		err := runner.RunPipelineWithLogAndFile(ctx, pipeline, job, logFile, pipelineFile)
		if err != nil {
			exitCode = 1
			failedPipeline = pipeline.Name

			var errorLog runner.ExecError
			if errors.As(err, &errorLog) && errorLog.Len() > 0 {
				fmt.Fprintf(os.Stderr, "\nAn error occurred in %q pipeline:\n\n", failedPipeline)
				fmt.Fprintf(os.Stderr, "  Exit code: %d\n", errorLog.LastExitCode)
				fmt.Fprintf(os.Stderr, "  Error output:\n")
				// Indent the error output
				for _, line := range strings.Split(errorLog.Output, "\n") {
					if line != "" {
						fmt.Fprintf(os.Stderr, "    %s\n", line)
					}
				}
				fmt.Fprintf(os.Stderr, "\n")
				fmt.Fprintf(os.Stderr, "  Stack trace:\n")
				fmt.Fprintf(os.Stderr, "%s", errorLog.Trace)
				exitCode = errorLog.LastExitCode
			}

			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}
	}

}
