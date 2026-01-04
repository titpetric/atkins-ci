package runner

import (
	"fmt"
	"strings"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/model"
)

// ListPipeline displays a pipeline's job tree with dependencies
func ListPipeline(pipeline *model.Pipeline) {
	allJobs := pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = pipeline.Tasks
	}

	fmt.Printf("%s\n", colors.BrightWhite(pipeline.Name))

	// Get jobs in dependency order
	jobOrder, err := ResolveJobDependencies(allJobs, "")
	if err != nil {
		fmt.Printf("%s %s\n", colors.BrightRed("ERROR:"), err)
		return
	}

	// Display jobs in tree format
	for i, jobName := range jobOrder {
		job := allJobs[jobName]
		isLast := i == len(jobOrder)-1

		// Prefix for tree structure
		prefix := "├─ "
		if isLast {
			prefix = "└─ "
		}

		// Job name in orange
		jobLabel := colors.BrightOrange(jobName)

		// Add dependencies on same line if any
		deps := GetDependencies(job.DependsOn)
		if len(deps) > 0 {
			depItems := make([]string, len(deps))
			for j, dep := range deps {
				depItems[j] = colors.BrightOrange(dep)
			}
			depsStr := strings.Join(depItems, ", ")
			jobLabel = jobLabel + fmt.Sprintf(" (depends_on: %s)", depsStr)
		}

		fmt.Printf("%s%s\n", prefix, jobLabel)

		// Show steps with their commands in orange
		if len(job.Steps) > 0 {
			stepPrefix := "│  "
			if isLast {
				stepPrefix = "   "
			}

			for j, step := range job.Steps {
				isLastStep := j == len(job.Steps)-1
				stepTreePrefix := "├─ "
				if isLastStep {
					stepTreePrefix = "└─ "
				}

				stepFullPrefix := stepPrefix + stepTreePrefix

				// Get the command to display
				cmd := ""
				if step.Run != "" {
					cmd = step.Run
				} else if step.Cmd != "" {
					cmd = step.Cmd
				} else if len(step.Cmds) > 0 {
					cmd = strings.Join(step.Cmds, " && ")
				}

				if cmd != "" {
					label := colors.White(cmd)
					if step.Deferred {
						label = label + " " + colors.BrightCyan("(deferred)")
					}
					fmt.Printf("%s%s\n", stepFullPrefix, label)
				} else if step.Name != "" {
					fmt.Printf("%s%s\n", stepFullPrefix, step.Name)
				}
			}
		}
	}
}
