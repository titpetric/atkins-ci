package main

import (
	"context"
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/titpetric/atkins/colors"
	"github.com/titpetric/atkins/runner"
)

// Version information injected at build time via ldflags
var (
	Version    = "dev"
	Commit     = "unknown"
	CommitTime = "unknown"
	Branch     = "unknown"
	Modified   = "false"
)

func fatalf(message string, args ...any) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func fatal(message string) {
	fatalf("%s", message)
}

func printBuildInfo() {
	fmt.Printf("atkins\n")
	fmt.Printf("  Version:     %s\n", Version)

	// Print VCS information
	if Commit != "unknown" {
		shortCommit := Commit
		if len(Commit) > 12 {
			shortCommit = Commit[:12]
		}
		fmt.Printf("  Commit:      %s\n", shortCommit)
	}

	if CommitTime != "unknown" {
		fmt.Printf("  CommitTime:  %s\n", CommitTime)
	}

	if Branch != "unknown" {
		fmt.Printf("  Branch:      %s\n", Branch)
	}

	if Modified == "true" {
		fmt.Printf("  Modified:    true (dirty working tree)\n")
	}

	// Get build info from embedded metadata
	exePath, err := os.Executable()
	var bi *buildinfo.BuildInfo
	if err == nil {
		bi, _ = buildinfo.ReadFile(exePath)
	}

	if bi != nil {
		// Module information
		fmt.Printf("  Module:      %s\n", bi.Path)
		if bi.Main.Path != "" {
			fmt.Printf("  MainModule:  %s\n", bi.Main.Path)
		}
		if bi.Main.Sum != "" {
			fmt.Printf("  Sum:         %s\n", bi.Main.Sum)
		}

		// Go version
		if bi.GoVersion != "" {
			fmt.Printf("  GoVersion:   %s\n", bi.GoVersion)
		}

		// Extract OS/Arch from settings
		var osVal, archVal string
		for _, setting := range bi.Settings {
			if setting.Key == "GOOS" {
				osVal = setting.Value
			} else if setting.Key == "GOARCH" {
				archVal = setting.Value
			}
		}
		if osVal != "" && archVal != "" {
			fmt.Printf("  OS/Arch:     %s/%s\n", osVal, archVal)
		}

		// Print all build settings for reference
		if len(bi.Settings) > 0 {
			fmt.Printf("\n  Build Settings:\n")
			for _, setting := range bi.Settings {
				// Skip GOOS/GOARCH as we already printed them
				if setting.Key == "GOOS" || setting.Key == "GOARCH" {
					continue
				}
				fmt.Printf("    %s=%s\n", setting.Key, setting.Value)
			}
		}
	}
}

func main() {
	var pipelineFile string
	var job string
	var listFlag bool
	var lintFlag bool
	var debug bool
	var logFile string
	var versionFlag bool

	flag.StringVar(&pipelineFile, "file", "atkins.yml", "Path to pipeline file")
	flag.StringVar(&job, "job", "", "Specific job to run")
	flag.BoolVar(&listFlag, "l", false, "List pipeline jobs and dependencies")
	flag.BoolVar(&lintFlag, "lint", false, "Lint pipeline for errors")
	flag.BoolVar(&debug, "debug", false, "Print debug data")
	flag.BoolVar(&versionFlag, "v", false, "Print version and build information")
	flag.StringVar(&logFile, "log", "", "Log file path for command execution (e.g., atkins.log)")
	flag.Parse()

	// Handle version flag
	if versionFlag {
		printBuildInfo()
		return
	}

	// Handle positional arguments
	args := flag.Args()
	for _, arg := range args {
		// Check if arg is a file that exists (shebang invocation)
		if _, err := os.Stat(arg); err == nil {
			pipelineFile = arg
		} else if arg == "lint" {
			lintFlag = true
		} else if arg == "-l" {
			listFlag = true
		} else if job == "" {
			// Treat as job name if not already set
			job = arg
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
			if errors.As(err, &errorLog) {
				if errorLog.Len() > 0 {
					fmt.Fprintf(os.Stderr, "\nAn error occurred in %q pipeline:\n\n", failedPipeline)
					fmt.Fprintf(os.Stderr, "  Exit code: %d\n", errorLog.LastExitCode)
					fmt.Fprintf(os.Stderr, "  Error output:\n")
					// Indent the error output
					for _, line := range strings.Split(errorLog.Output, "\n") {
						if line != "" {
							fmt.Fprintf(os.Stderr, "    %s\n", line)
						}
					}
				}
				exitCode = errorLog.LastExitCode
			} else {
				// Not an ExecError, print it as is
				fmt.Fprintf(os.Stderr, "\nAn error occurred in %q pipeline:\n", failedPipeline)
				fmt.Fprintf(os.Stderr, "  %s\n", err.Error())
			}

			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}
	}
}
