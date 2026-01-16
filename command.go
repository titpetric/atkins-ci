package main

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/titpetric/cli"
	yaml "gopkg.in/yaml.v3"

	"github.com/titpetric/atkins/colors"
	"github.com/titpetric/atkins/runner"
)

func NewCommand() *cli.Command {
	var pipelineFile string
	var job string
	var listFlag bool
	var lintFlag bool
	var debug bool
	var logFile string
	var versionFlag bool
	var fileFlag *pflag.Flag

	return &cli.Command{
		Name:    "run",
		Title:   "Pipeline automation tool",
		Default: true,
		Bind: func(fs *pflag.FlagSet) {
			fs.StringVarP(&pipelineFile, "file", "f", "", "Path to pipeline file (auto-discovers .atkins.yml)")
			fs.StringVar(&job, "job", "", "Specific job to run")
			fs.BoolVarP(&listFlag, "list", "l", false, "List pipeline jobs and dependencies")
			fs.BoolVar(&lintFlag, "lint", false, "Lint pipeline for errors")
			fs.BoolVar(&debug, "debug", false, "Print debug data")
			fs.BoolVarP(&versionFlag, "version", "v", false, "Print version and build information")
			fs.StringVar(&logFile, "log", "", "Log file path for command execution")
			fileFlag = fs.Lookup("file")
		},
		Run: func(ctx context.Context, args []string) error {
			// Handle version flag
			if versionFlag {
				printVersionInfo()
				return nil
			}

			// Track if file was explicitly provided
			fileExplicitlySet := fileFlag != nil && fileFlag.Changed

			// Handle positional arguments
			for _, arg := range args {
				// Check if arg is a file that exists (shebang invocation)
				if _, err := os.Stat(arg); err == nil {
					pipelineFile = arg
					fileExplicitlySet = true
				} else if arg == "-l" {
					listFlag = true
				} else if job == "" {
					// Treat as job name if not already set
					job = arg
				}
			}

			var absPath string
			var err error

			if fileExplicitlySet {
				// If -f/--file was explicitly provided, use it directly without changing workdir
				absPath, err = filepath.Abs(pipelineFile)
				if err != nil {
					return fmt.Errorf("%s %v", colors.BrightRed("ERROR:"), err)
				}
			} else {
				// Discover config file by traversing parent directories
				configPath, configDir, err := runner.DiscoverConfigFromCwd()
				if err != nil {
					return fmt.Errorf("%s %v", colors.BrightRed("ERROR:"), err)
				}
				absPath = configPath
				pipelineFile = configPath

				// Change to the directory containing the config file
				if err := os.Chdir(configDir); err != nil {
					return fmt.Errorf("%s failed to change directory to %s: %v", colors.BrightRed("ERROR:"), configDir, err)
				}
			}

			// Load and parse pipeline
			pipelines, err := runner.LoadPipeline(absPath)
			if err != nil {
				return fmt.Errorf("%s %s", colors.BrightRed("ERROR:"), err)
			}

			if len(pipelines) == 0 {
				return fmt.Errorf("%s No pipelines found", colors.BrightRed("ERROR:"))
			}

			// Handle lint mode
			if lintFlag {
				for _, pipeline := range pipelines {
					linter := runner.NewLinter(pipeline)
					lintErrors := linter.Lint()
					if len(lintErrors) > 0 {
						fmt.Printf("%s Pipeline '%s' has errors:\n", colors.BrightRed("✗"), pipeline.Name)
						for _, lintErr := range lintErrors {
							fmt.Printf("  %s: %s\n", lintErr.Job, lintErr.Detail)
						}
						os.Exit(1)
					}
				}
				fmt.Printf("%s Pipeline '%s' is valid\n", colors.BrightGreen("✓"), pipelines[0].Name)
				return nil
			}

			// Handle list mode
			if listFlag {
				for _, pipeline := range pipelines {
					linter := runner.NewLinter(pipeline)
					lintErrors := linter.Lint()
					if len(lintErrors) > 0 {
						fmt.Printf("%s Pipeline '%s' has dependency errors:\n", colors.BrightRed("✗"), pipeline.Name)
						for _, lintErr := range lintErrors {
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
				return nil
			}

			// Run pipeline(s)
			var exitCode int
			var failedPipeline string

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
							for _, line := range strings.Split(errorLog.Output, "\n") {
								if line != "" {
									fmt.Fprintf(os.Stderr, "    %s\n", line)
								}
							}
						}
						exitCode = errorLog.LastExitCode
					} else {
						fmt.Fprintf(os.Stderr, "\nAn error occurred in %q pipeline:\n", failedPipeline)
						fmt.Fprintf(os.Stderr, "  %s\n", err.Error())
					}

					if exitCode != 0 {
						os.Exit(exitCode)
					}
				}
			}
			return nil
		},
	}
}

func printVersionInfo() {
	fmt.Printf("atkins\n")
	fmt.Printf("  Version:     %s\n", Version)

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

	exePath, err := os.Executable()
	var bi *buildinfo.BuildInfo
	if err == nil {
		bi, _ = buildinfo.ReadFile(exePath)
	}

	if bi != nil {
		fmt.Printf("  Module:      %s\n", bi.Path)
		if bi.Main.Path != "" {
			fmt.Printf("  MainModule:  %s\n", bi.Main.Path)
		}
		if bi.Main.Sum != "" {
			fmt.Printf("  Sum:         %s\n", bi.Main.Sum)
		}

		if bi.GoVersion != "" {
			fmt.Printf("  GoVersion:   %s\n", bi.GoVersion)
		}

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

		if len(bi.Settings) > 0 {
			fmt.Printf("\n  Build Settings:\n")
			for _, setting := range bi.Settings {
				if setting.Key == "GOOS" || setting.Key == "GOARCH" {
					continue
				}
				fmt.Printf("    %s=%s\n", setting.Key, setting.Value)
			}
		}
	}
}
